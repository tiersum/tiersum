package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/viper"
)

// LocalProvider implements Provider for local/Ollama models
type LocalProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewLocalProvider creates a new local/Ollama provider
func NewLocalProvider() *LocalProvider {
	return &LocalProvider{
		baseURL: viper.GetString("llm.local.base_url"),
		model:   viper.GetString("llm.local.model"),
		client:  &http.Client{},
	}
}

// ollamaRequest represents the request body
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaResponse represents the response body
type ollamaResponse struct {
	Response string `json:"response"`
}

// Summarize summarizes content using local/Ollama model
func (p *LocalProvider) Summarize(ctx context.Context, content string, maxLength int) (string, error) {
	prompt := fmt.Sprintf("Summarize the following content in %d characters or less:\n\n%s", maxLength, content)

	reqBody := ollamaRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}
