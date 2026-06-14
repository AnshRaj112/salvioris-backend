package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func LLMEnabled() bool {
	return strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != ""
}

func SummarizeWithLLM(prompt string) (string, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return "", fmt.Errorf("llm not configured")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	log.Printf("[LLM Gemini] Summarize request received. Model: %s, Prompt length: %d", model, len(prompt))

	body, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a clinical documentation assistant. Never diagnose. Provide brief supportive summaries only."},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 400,
	})

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		log.Printf("[LLM Gemini] Error creating request: %v", err)
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[LLM Gemini] HTTP request failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Printf("[LLM Gemini] API returned error status %d: %s", resp.StatusCode, string(data))
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
		log.Printf("[LLM Gemini] Failed to parse API response: %v", err)
		return "", fmt.Errorf("invalid llm response")
	}
	log.Printf("[LLM Gemini] Successfully generated summary of length: %d", len(out.Choices[0].Message.Content))
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func LogLLMStatus() {
	if LLMEnabled() {
		log.Println("✅ LLM Gemini configured")
	} else {
		log.Println("⚠️  LLM Gemini not configured (set GEMINI_API_KEY)")
	}
}
