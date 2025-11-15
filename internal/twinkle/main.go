package twinkle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type TwinkleRequest struct {
	Prompt string `json:"prompt"`
}

type TwinkleResponse struct {
	Completion string `json:"completion"`
}

func CallTwinkleAPI(prompt string) (string, error) {
	url := "xx" // TODO replace with the actual URL from config

	// 创建请求体
	requestBody, err := json.Marshal(TwinkleRequest{
		Prompt: prompt,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %v", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var twinkleResponse TwinkleResponse
	if err := json.Unmarshal(body, &twinkleResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	return twinkleResponse.Completion, nil
}
