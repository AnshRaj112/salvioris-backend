package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func LLMEnabled() bool {
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
}

func SummarizeWithLLM(prompt string) (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return "", fmt.Errorf("llm not configured")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model": os.Getenv("OPENAI_MODEL"),
		"messages": []map[string]string{
			{"role": "system", "content": "You are a clinical documentation assistant. Never diagnose. Provide brief supportive summaries only."},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 400,
	})
	if os.Getenv("OPENAI_MODEL") == "" {
		body, _ = json.Marshal(map[string]interface{}{
			"model": "gpt-4o-mini",
			"messages": []map[string]string{
				{"role": "system", "content": "You are a clinical documentation assistant. Never diagnose. Provide brief supportive summaries only."},
				{"role": "user", "content": prompt},
			},
			"max_tokens": 400,
		})
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai: %s", string(data))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("invalid llm response")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
