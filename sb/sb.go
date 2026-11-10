// Package sb is a minimal client for the local SonnenBatterie API (v1 and v2).
// It only logs in and reads (GET); it never changes anything on the battery.
package sb

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

type SonnenBatterie struct {
	Username string
	Password string
	Host     string
	BaseUrl  string
	Token    string
	client   *http.Client
}

// Result of one GET request: the HTTP status and either the decoded JSON
// body (object, list, ...) or an error message.
type Result struct {
	Status int         `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func SunBatInit(username, password string, host string) *SonnenBatterie {
	//goland:noinspection HttpUrlsUsage
	return &SonnenBatterie{
		Username: username,
		Password: password,
		Host:     host,
		BaseUrl:  "http://" + host + "/api/",
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Login obtains a session token. Firmware >= 1.18 answers the salt request
// and uses the salted login; older firmware the plain challenge/response.
func (sb *SonnenBatterie) Login() error {
	resp, err := sb.client.Get(sb.BaseUrl + "salt/" + sb.Username)
	if err != nil {
		return fmt.Errorf("battery not reachable: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return sb.createSessionTokenNew()
	}
	return sb.createSessionTokenOld()
}

func (sb *SonnenBatterie) createSessionTokenOld() error {
	pwSha512 := sha512.Sum512([]byte(sb.Password))
	pwHex512 := hex.EncodeToString(pwSha512[:])

	body, _, err := sb.read(sb.client.Get(sb.BaseUrl + "challenge"))
	if err != nil {
		return err
	}
	// the challenge comes as a JSON string, including the quotes
	challenge := strings.Trim(strings.TrimSpace(string(body)), `"`)
	response := hex.EncodeToString(pbkdf2.Key([]byte(pwHex512), []byte(challenge), 7500, 64, sha512.New))
	return sb.postSession(challenge, response, nil)
}

func (sb *SonnenBatterie) createResponseFromValues(password, challenge, salt string) string {
	// SHA-512 of the password (hex), PBKDF2 with the salt, then HMAC-SHA256 of the challenge
	pwSha512 := sha512.Sum512([]byte(password))
	pwHex512 := hex.EncodeToString(pwSha512[:])
	dk := pbkdf2.Key([]byte(pwHex512), []byte(salt), 7500, 64, sha512.New)
	h := hmac.New(sha256.New, []byte(hex.EncodeToString(dk)))
	h.Write([]byte(challenge))
	return hex.EncodeToString(h.Sum(nil))
}

func (sb *SonnenBatterie) createSessionTokenNew() error {
	body, _, err := sb.read(sb.client.Get(sb.BaseUrl + "challenge"))
	if err != nil {
		return err
	}
	// the challenge may be a JSON string, a JSON object or plain text
	var challenge string
	var decoded interface{}
	if json.Unmarshal(body, &decoded) == nil {
		switch v := decoded.(type) {
		case string:
			challenge = v
		case map[string]interface{}:
			if s, ok := v["challenge"].(string); ok {
				challenge = s
			} else {
				for _, val := range v {
					if s, ok := val.(string); ok {
						challenge = s
						break
					}
				}
			}
		}
	}
	if challenge == "" {
		challenge = strings.TrimSpace(string(body))
	}

	body, _, err = sb.read(sb.client.Get(sb.BaseUrl + "salt/" + sb.Username))
	if err != nil {
		return err
	}
	var saltMap map[string]interface{}
	if err := json.Unmarshal(body, &saltMap); err != nil {
		return fmt.Errorf("unexpected salt response: %w", err)
	}
	salt, _ := saltMap["salt"].(string)

	// the battery may ask to repeat the login with a new challenge
	for i := 0; i < 2; i++ {
		var newChallenge string
		err := sb.postSession(challenge, sb.createResponseFromValues(sb.Password, challenge, salt), &newChallenge)
		if err != nil || newChallenge == "" {
			return err
		}
		challenge = newChallenge
	}
	return fmt.Errorf("failed to obtain authentication_token")
}

// postSession sends the login response. On a "new_challenge" reply it
// stores the new challenge (if requested) instead of a token.
func (sb *SonnenBatterie) postSession(challenge, response string, newChallenge *string) error {
	payload := url.Values{}
	payload.Set("user", sb.Username)
	payload.Set("challenge", challenge)
	payload.Set("response", response)
	resp, err := sb.client.Post(sb.BaseUrl+"session", "application/x-www-form-urlencoded",
		bytes.NewBufferString(payload.Encode()))
	body, status, err := sb.read(resp, err)
	if err != nil {
		return err
	}
	if status >= 400 {
		// the reply may contain a block_time after too many failed logins
		return fmt.Errorf("login failed with HTTP %d: %s", status, strings.TrimSpace(string(body)))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("unexpected session response: %w", err)
	}
	if s, ok := result["new_challenge"].(string); ok && s != "" && newChallenge != nil {
		*newChallenge = s
		return nil
	}
	if token, ok := result["authentication_token"].(string); ok && token != "" {
		sb.Token = token
		return nil
	}
	if e, ok := result["error"]; ok {
		return fmt.Errorf("login error: %v", e)
	}
	return fmt.Errorf("unexpected session response")
}

// Get reads one endpoint, path relative to /api/ (e.g. "battery_system",
// "v2/latestdata"). Any JSON body is accepted (objects and lists).
func (sb *SonnenBatterie) Get(path string) Result {
	req, err := http.NewRequest("GET", sb.BaseUrl+path, nil)
	if err != nil {
		return Result{Error: err.Error()}
	}
	req.Header.Set("Auth-Token", sb.Token)
	body, status, err := sb.read(sb.client.Do(req))
	if err != nil {
		return Result{Status: status, Error: err.Error()}
	}

	var data interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber() // keep numbers exactly as the battery sent them
	jsonErr := dec.Decode(&data)

	if status >= 400 {
		msg := strings.TrimSpace(string(body))
		if obj, ok := data.(map[string]interface{}); ok && jsonErr == nil && obj["error"] != nil {
			msg = fmt.Sprint(obj["error"])
		}
		return Result{Status: status, Error: msg}
	}
	if jsonErr != nil {
		return Result{Status: status, Error: "invalid JSON: " + jsonErr.Error()}
	}
	return Result{Status: status, Data: data}
}

func (sb *SonnenBatterie) read(resp *http.Response, err error) ([]byte, int, error) {
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
