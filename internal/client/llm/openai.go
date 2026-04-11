// Package llm implements third-party client layer
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/client"
)

// OpenAIProvider implements client.ILLMProvider for OpenAI
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
	Model              string         `json:"model"`
	Messages           []message      `json:"messages"`
	MaxTokens          int            `json:"max_tokens"`
	Temperature        float64        `json:"temperature"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	// Thinking is Moonshot Kimi K2 API: {"type":"disabled"} turns off chain-of-thought for compatible models.
	Thinking map[string]any `json:"thinking,omitempty"`
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
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Generate implements ILLMProvider.Generate
func (p *OpenAIProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	// Get temperature from config, default to 0.3
	temperature := viper.GetFloat64("llm.openai.temperature")
	if temperature == 0 {
		temperature = 0.3
	}

	// Some models (like kimi-k2.5) only support temperature=1
	// Check if we need to override for specific models
	if strings.Contains(p.model, "kimi-k2") || strings.Contains(p.model, "k2.5") {
		temperature = 1.0
	}

	reqBody := openAIRequest{
		Model: p.model,
		Messages: []message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:          maxTokens,
		Temperature:        temperature,
		ChatTemplateKwargs: openAIThinkOffChatTemplate(p.baseURL, p.model),
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
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for API error
	if result.Error != nil {
		return "", fmt.Errorf("api error: %s - %s", result.Error.Type, result.Error.Message)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}

// openAIThinkOffChatTemplate returns chat_template_kwargs for OpenAI-compatible APIs that support disabling CoT.
func openAIThinkOffChatTemplate(baseURL, model string) map[string]any {
	if !disableThinkEnabled() {
		return nil
	}
	u := strings.ToLower(baseURL)
	m := strings.ToLower(model)
	if strings.Contains(u, "deepseek.com") || strings.Contains(m, "deepseek") {
		return map[string]any{"enable_thinking": false}
	}
	if strings.Contains(u, "dashscope.aliyuncs.com") || strings.Contains(u, "dashscope") {
		return map[string]any{"enable_thinking": false}
	}
	if strings.Contains(u, "siliconflow.cn") {
		return map[string]any{"enable_thinking": false}
	}
	return nil
}

var _ client.ILLMProvider = (*OpenAIProvider)(nil)
