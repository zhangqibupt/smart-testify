package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"smart-testify/internal/copilot"
	"strings"
	"time"
)

var copilotClient *copilot.Client

func getCopilotClient() *copilot.Client {
	if copilotClient == nil {
		copilotClient = copilot.NewCopilotClient(getGlobalConfig().CopilotToken, false)
	}

	return copilotClient
}

var initTokenCmd = &cobra.Command{
	Use:   "init-token",
	Short: "Initialize Copilot token",
	Run: func(cmd *cobra.Command, args []string) {
		if err := InitCopilotToken(); err != nil {
			log.Errorf("Failed to initialize Copilot token: %v", err)
		}
	},
}

var copilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Configure settings for Copilot.",
}

func init() {
	copilotCmd.AddCommand(initTokenCmd)
}

func InitCopilotToken() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	token, err := GetCopilotToken()
	if err != nil {
		return err
	}

	config.CopilotToken = token
	if err := saveConfig(config); err != nil {
		return err
	}

	log.Infof("Copilot token initialized successfully")
	return nil
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

	log.Error("Failed to get access token after %s retries", maxRetry)
	return "", errors.New("failed to get access token")
}
