package sb

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "fmt"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"net/http"
	"net/url"
	"os"
)

type SonnenBatterie struct {
	Username string
	Password string
	Host     string
	BaseUrl  string
	Token    string
}

func SunBatInit(username, password string, host string) *SonnenBatterie {
	return &SonnenBatterie{
		Username: username,
		Password: password,
		Host:     host,
		BaseUrl:  "http://" + host + "/api/",
	}
}

func (sb *SonnenBatterie) Login() {
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
