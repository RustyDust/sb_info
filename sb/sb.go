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
	"os"

	"golang.org/x/crypto/pbkdf2"
)

type SonnenBatterie struct {
	Username string
	Password string
	Host     string
	BaseUrl  string
	BaseApi2 string
	Token    string
}

func SunBatInit(username, password string, host string) *SonnenBatterie {
	//goland:noinspection HttpUrlsUsage
	return &SonnenBatterie{
		Username: username,
		Password: password,
		Host:     host,
		BaseUrl:  "http://" + host + "/api/",
		BaseApi2: "http://" + host + "/api/v2/",
	}
}

func (sb *SonnenBatterie) Login() {
	// Check if new login method is supported
	salt, err := http.Get(sb.BaseUrl + "salt/" + sb.Username)
	if err == nil && salt.StatusCode == 200 {
		// Use new login method
		sb.createSessionTokenNew()
	} else {
		// Use old login method
		pwSha512 := sha512.Sum512([]byte(sb.Password))
		pwHex512 := hex.EncodeToString(pwSha512[:])
		// fmt.Printf("pwHex: %s (%d)\n", pwHex512, len(pwHex512))

		req, err := http.Get(sb.BaseUrl + "challenge")
		if err != nil {
			panic(err)
		}
		challenge, err := io.ReadAll(req.Body)
		// yikes, the stream also reads the opening and closing quotes
		challenge = challenge[1 : len(challenge)-1]
		if err != nil {
			panic(err)
		}
		// fmt.Printf("challenge: %s\n", string(challenge))

		response := hex.EncodeToString(pbkdf2.Key([]byte(pwHex512), challenge, 7500, 64, sha512.New))
		// fmt.Printf("response: ->%s<- (%d)\n", response, len(response))

		payload := url.Values{}
		payload.Set("user", sb.Username)
		payload.Set("challenge", string(challenge))
		payload.Set("response", response)

		// fmt.Printf("\nPayload: %s\n", payload.Encode())

		req, err = http.Post(sb.BaseUrl+"session", "application/x-www-form-urlencoded", bytes.NewBufferString(payload.Encode()))
		if err != nil {
			panic(err)
		}
		session, err := io.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}

		var result map[string]interface{}
		err = json.Unmarshal(session, &result)
		if err != nil {
			panic(err)
		}
		if result["error"] != nil {
			fmt.Printf("!!! - login error: %s\n\n", result["error"])
			os.Exit(1)
		}
		// fmt.Printf("r: %s\n", result["authentication_token"])

		sb.Token = result["authentication_token"].(string)
	}
}

func (sb *SonnenBatterie) createResponseFromValues(username, password, challenge, salt string) string {
	// Step 1: SHA-512 hash the password and convert to hex
	pwSha512 := sha512.Sum512([]byte(password))
	pwHex512 := hex.EncodeToString(pwSha512[:])

	// Step 2: Derive key using PBKDF2 with salt
	dk := pbkdf2.Key([]byte(pwHex512), []byte(salt), 7500, 64, sha512.New)
	derivedHex := hex.EncodeToString(dk)

	// Step 3: Create HMAC-SHA256 of challenge using derived key
	h := hmac.New(sha256.New, []byte(derivedHex))
	h.Write([]byte(challenge))
	response := hex.EncodeToString(h.Sum(nil))

	return response
}

func (sb *SonnenBatterie) createSessionTokenNew() {
	// Step 1: Get challenge
	req, err := http.Get(sb.BaseUrl + "challenge")
	if err != nil {
		panic(err)
	}
	challengeData, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	req.Body.Close()

	// Parse challenge (could be JSON or plain text)
	var challenge string
	var challengeMap map[string]interface{}
	if err := json.Unmarshal(challengeData, &challengeMap); err == nil {
		// It's JSON
		if val, ok := challengeMap["challenge"].(string); ok {
			challenge = val
		} else {
			// Get first value from map
			for _, v := range challengeMap {
				if s, ok := v.(string); ok {
					challenge = s
					break
				}
			}
		}
	} else {
		// Plain text
		challenge = string(challengeData)
	}

	// Step 2: Get salt
	req, err = http.Get(sb.BaseUrl + "salt/" + sb.Username)
	if err != nil {
		panic(err)
	}
	saltData, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	req.Body.Close()

	var saltMap map[string]interface{}
	err = json.Unmarshal(saltData, &saltMap)
	if err != nil {
		panic(err)
	}
	salt := saltMap["salt"].(string)

	// Step 3: Loop up to 2 times to handle new_challenge
	for i := 0; i < 2; i++ {
		response := sb.createResponseFromValues(sb.Username, sb.Password, challenge, salt)

		payload := url.Values{}
		payload.Set("user", sb.Username)
		payload.Set("challenge", challenge)
		payload.Set("response", response)

		req, err = http.Post(sb.BaseUrl+"session", "application/x-www-form-urlencoded", bytes.NewBufferString(payload.Encode()))
		if err != nil {
			panic(err)
		}

		if req.StatusCode >= 400 {
			body, _ := io.ReadAll(req.Body)
			req.Body.Close()
			fmt.Printf("!!! - Login failed with HTTP %d\n", req.StatusCode)
			fmt.Printf("Server reply: %s\n", string(body))
			os.Exit(1)
		}

		sessionData, err := io.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		req.Body.Close()

		var result map[string]interface{}
		err = json.Unmarshal(sessionData, &result)
		if err != nil {
			panic(err)
		}

		// Check for new_challenge and retry if present
		if newChallenge, ok := result["new_challenge"].(string); ok && newChallenge != "" {
			challenge = newChallenge
			continue
		}

		// Check for authentication_token
		if token, ok := result["authentication_token"].(string); ok && token != "" {
			sb.Token = token
			return
		}

		// Unexpected response
		jsonStr, _ := json.Marshal(result)
		fmt.Printf("!!! - Unexpected /session response: %s\n", string(jsonStr))
		os.Exit(1)
	}

	fmt.Println("!!! - Failed to obtain authentication_token")
	os.Exit(1)
}

func (sb *SonnenBatterie) Get(what string) (string, bool) {
	req, err := http.NewRequest("GET", sb.BaseUrl+what, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Auth-Token", sb.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	// fmt.Printf("contents:\n%v\n", string(contents))

	// map JSON
	var result map[string]interface{}
	err = json.Unmarshal(contents, &result)
	if err != nil {
		return "", false
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", false
	}

	// fmt.Println(string(out))
	return string(out), true
}

func (sb *SonnenBatterie) GetApi2(what string) (string, bool) {
	req, err := http.NewRequest("GET", sb.BaseApi2+what, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Auth-Token", sb.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	// fmt.Printf("contents:\n%v\n", string(contents))

	// map JSON
	var result map[string]interface{}
	err = json.Unmarshal(contents, &result)
	if err != nil {
		return "", false
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", false
	}

	// fmt.Println(string(out))
	return string(out), true
}
