// Package llm implements third-party client layer
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/client"
)

// OllamaProvider implements client.ILLMProvider for local Ollama models
type OllamaProvider struct {
	baseURL string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider() *OllamaProvider {
	timeout := viper.GetDuration("llm.local.timeout")
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OllamaProvider{
		baseURL: viper.GetString("llm.local.base_url"),
		model:   viper.GetString("llm.local.model"),
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type ollamaRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Options struct {
		NumPredict  int     `json:"num_predict,omitempty"`
		Temperature float64 `json:"temperature,omitempty"`
	} `json:"options"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate implements ILLMProvider.Generate
func (p *OllamaProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	reqBody := ollamaRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false,
	}
	reqBody.Options.NumPredict = maxTokens
	reqBody.Options.Temperature = 0.3

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

	if result.Done {
		return result.Response, nil
	}

	return "", fmt.Errorf("no response from Ollama")
}

var _ client.ILLMProvider = (*OllamaProvider)(nil)
