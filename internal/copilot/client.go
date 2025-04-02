package copilot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"smart-testify/internal/logger"
	"strings"
	"time"
)

const model = "gpt-4o"

var log = logger.GetLogger() // Global logger

// Client struct holds the token and messages for interactions
type Client struct {
	Token      string
	Contextual bool // Whether to append the previous messages to the current request
	Messages   []map[string]string
}

// NewCopilotClient initializes a Client instance
func NewCopilotClient(token string, Contextual bool) *Client {
	return &Client{
		Contextual: Contextual,
		Token:      token,
	}
}

func GetCopilotToken() (string, error) {
	clientID := "Iv1.b507a08c87ecfe98"
	scope := "read:user"
	url := "https://github.com/login/device/code"

	reqBody := fmt.Sprintf(`{"client_id":"%s","scope":"%s"}`, clientID, scope)

	// Create the initial request with Accept header
	req, err := http.NewRequest("POST", url, strings.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")       // Add Accept header
	req.Header.Set("Content-Type", "application/json") // Add Content-Type header

	// Create a custom HTTP client with a longer timeout (e.g., 60 seconds)
	client := &http.Client{
		Timeout: 60 * time.Second, // Set timeout to 60 seconds
	}
	log.Infof("Requesting access token from GitHub Copilot...")
	// Send the initial request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the entire response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body from github: %v", err)
	}

	var respJSON map[string]interface{}
	if err := json.Unmarshal(respBody, &respJSON); err != nil {
		return "", err
	}

	// Extract the necessary fields from the response
	deviceCode, ok := respJSON["device_code"].(string)
	if !ok {
		return "", errors.New("device_code not found in response")
	}
	userCode, ok := respJSON["user_code"].(string)
	if !ok {
		return "", errors.New("user_code not found in response")
	}
	verificationURI, ok := respJSON["verification_uri"].(string)
	if !ok {
		return "", errors.New("verification_uri not found in response")
	}

	// Inform the user to authenticate
	log.Infof("Please visit %s and enter code %s to authenticate.\n", verificationURI, userCode)

	maxRetry := 10
	// Wait for the user to authenticate
	for maxRetry > 0 {
		maxRetry--
		time.Sleep(5 * time.Second)

		// Request the access token
		tokenURL := "https://github.com/login/oauth/access_token"
		tokenReqBody := fmt.Sprintf(`{"client_id":"%s","device_code":"%s","grant_type":"urn:ietf:params:oauth:grant-type:device_code"}`, clientID, deviceCode)

		// Create the token request with Accept header
		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(tokenReqBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")       // Add Accept header
		req.Header.Set("Content-Type", "application/json") // Add Content-Type header

		// Send the request for the access token
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var respJSON map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
			return "", err
		}

		// If we got the access token, save it and break out of the loop
		accessToken, ok := respJSON["access_token"].(string)
		if ok {
			return accessToken, nil
		}
	}

	log.Errorf("Failed to get access token after %d retries", maxRetry)
	return "", errors.New("failed to get access token")
}

// Chat sends a message to the Copilot API and returns the assistant's response
func (c *Client) Chat(message string) (string, error) {
	if c.Token == "" {
		return "", errors.New("token is not initialized, please run 'smart-testify config copilot init-token' to initialize the token")
	}

	if c.Contextual {
		c.Messages = append(c.Messages, map[string]string{
			"content": message,
			"role":    "user",
		})
	} else {
		c.Messages = []map[string]string{
			{
				"content": message,
				"role":    "user",
			},
		}
	}

	chatURL := "https://api.githubcopilot.com/chat/completions"
	reqBody := map[string]interface{}{
		"intent":      false,
		"model":       model,
		"temperature": 0,
		"top_p":       1,
		"n":           1,
		"stream":      true,
		"messages":    c.Messages,
	}

	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", chatURL, strings.NewReader(string(reqBodyJSON)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Editor-Version", "vscode/1.80.1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the entire response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Convert the response body to a string and split by newlines
	result := ""
	respText := string(respBody)
	lines := strings.Split(respText, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "data: {") {
			// Parse the completion from the line as JSON
			var jsonCompletion map[string]interface{}
			if err := json.Unmarshal([]byte(line[6:]), &jsonCompletion); err != nil {
				continue // Skip invalid lines
			}

			// Extract the "choices" array from the parsed JSON
			choices, choicesExist := jsonCompletion["choices"].([]interface{})
			if choicesExist && len(choices) > 0 {
				choice := choices[0].(map[string]interface{})
				if delta, deltaExist := choice["delta"].(map[string]interface{}); deltaExist {
					if content, contentExist := delta["content"].(string); contentExist {
						result += content
					}
				}
			}
		}
	}

	// Append the assistant's response to messages
	c.Messages = append(c.Messages, map[string]string{
		"content": result,
		"role":    "assistant",
	})

	if result == "" {
		return "", fmt.Errorf("no response received, status code: %d, response body: %s", resp.StatusCode, respText)
	}

	// Append the assistant's response
	c.Messages = append(c.Messages, map[string]string{
		"content": result,
		"role":    "assistant",
	})

	return result, nil
}
