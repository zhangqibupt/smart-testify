package copilot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"smart-testify/internal/logger"
	"strings"
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
