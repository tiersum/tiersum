// Package llm implements third-party client layer
// External system dependencies, e.g., LLM client
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/ports"
)

// OpenAIProvider implements ports.LLMProvider for OpenAI
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  viper.GetString("llm.openai.api_key"),
		baseURL: viper.GetString("llm.openai.base_url"),
		model:   viper.GetString("llm.openai.model"),
		client:  &http.Client{},
	}
}

type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Generate implements ports.LLMProvider.Generate
func (p *OpenAIProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	reqBody := openAIRequest{
		Model: p.model,
		Messages: []message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.3,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}

// Compile-time interface check
var _ ports.LLMProvider = (*OpenAIProvider)(nil)
