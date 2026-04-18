// Package llm implements third-party client layer
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
	ResponseFormat     map[string]any `json:"response_format,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	// Thinking is Moonshot Kimi K2 API: {"type":"disabled"} turns off chain-of-thought for compatible models.
	Thinking map[string]any `json:"thinking,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatMessageContent supports both:
// - OpenAI chat.completions: "content": "..."
// - OpenAI-compatible variants: "content": [{ "type": "text", "text": "..." }, ...]
type chatMessageContent struct {
	Text string
}

func (c *chatMessageContent) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		c.Text = s
		return nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			sb.WriteString(p.Text)
		}
		c.Text = sb.String()
		return nil
	}
	// Unknown shape; keep empty and let callers decide.
	c.Text = ""
	return nil
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content chatMessageContent `json:"content"`
			// Moonshot Kimi returns content in this field when thinking is enabled.
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

var (
	onlyTemperatureRe = regexp.MustCompile(`(?i)invalid temperature:\s*only\s*([0-9]*\.?[0-9]+)\s*is allowed`)
)

// Generate implements ILLMProvider.Generate
func (p *OpenAIProvider) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	temperature := p.resolveTemperature()
	reqBody := openAIRequest{
		Model: p.model,
		Messages: []message{
			{Role: "system", Content: "You are a concise assistant. Respond briefly and accurately."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:          maxTokens,
		Temperature:        temperature,
		ResponseFormat:     openAIResponseFormat(p.baseURL, p.model),
		ChatTemplateKwargs: openAIThinkOffChatTemplate(p.baseURL, p.model),
		Thinking:           openAIThinkingField(p.baseURL, p.model),
	}
	// Retry once when the backend enforces a single temperature value for this model.
	out, body, result, err := p.doChatCompletion(ctx, reqBody)
	if err != nil {
		if onlyTemp, ok := parseOnlyTemperature(err.Error()); ok {
			reqBody.Temperature = onlyTemp
			out2, _, _, err2 := p.doChatCompletion(ctx, reqBody)
			if err2 == nil {
				return out2, nil
			}
			return "", err2
		}
		return "", err
	}
	_ = body
	_ = result
	return out, nil
}

func (p *OpenAIProvider) resolveTemperature() float64 {
	// Get temperature from config, default to 0.3.
	t := viper.GetFloat64("llm.openai.temperature")
	if t == 0 {
		t = 0.3
	}
	// Moonshot Kimi sometimes enforces a fixed temperature per model; default to 0.6 when configured model suggests it.
	m := strings.ToLower(p.model)
	if strings.Contains(m, "kimi") || strings.Contains(m, "k2") {
		// If the user explicitly configured a non-zero temperature, keep it; otherwise pick a safer default.
		if viper.GetFloat64("llm.openai.temperature") == 0 {
			t = 0.6
		}
	}
	return t
}

func parseOnlyTemperature(errMsg string) (float64, bool) {
	m := onlyTemperatureRe.FindStringSubmatch(errMsg)
	if len(m) != 2 {
		return 0, false
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func (p *OpenAIProvider) doChatCompletion(ctx context.Context, reqBody openAIRequest) (string, []byte, openAIResponse, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, openAIResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", nil, openAIResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, openAIResponse{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return "", nil, openAIResponse{}, fmt.Errorf("read response body: %w", rerr)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return "", body, openAIResponse{}, fmt.Errorf("api error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result openAIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, openAIResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for API error
	if result.Error != nil {
		return "", body, result, fmt.Errorf("api error: %s - %s", result.Error.Type, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", body, result, fmt.Errorf("no response from OpenAI")
	}

	msg := result.Choices[0].Message
	out := strings.TrimSpace(msg.Content.Text)
	if out == "" {
		// OpenAI-compatible variants may return text in reasoning_content instead of content.
		out = strings.TrimSpace(msg.ReasoningContent)
	}
	if out == "" {
		snip := string(body)
		if len(snip) > 800 {
			snip = snip[:800] + "...(truncated)"
		}
		fr := strings.TrimSpace(result.Choices[0].FinishReason)
		if fr != "" {
			return "", body, result, fmt.Errorf("empty content from LLM (finish_reason=%s, body=%s)", fr, snip)
		}
		return "", body, result, fmt.Errorf("empty content from LLM (body=%s)", snip)
	}
	return out, body, result, nil
}

func openAIThinkingField(baseURL, model string) map[string]any {
	if !disableThinkEnabled() {
		return nil
	}
	u := strings.ToLower(baseURL)
	m := strings.ToLower(model)
	// Moonshot Kimi: prefer disabling thinking so final content lands in message.content.
	if strings.Contains(u, "moonshot") || strings.Contains(u, "moonshot.cn") || strings.Contains(m, "kimi") {
		return map[string]any{"type": "disabled"}
	}
	return nil
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

// openAIResponseFormat returns response_format for providers that support json_object (e.g. Moonshot Kimi)
// to reduce token waste from prose around the JSON.
func openAIResponseFormat(baseURL, model string) map[string]any {
	u := strings.ToLower(baseURL)
	m := strings.ToLower(model)
	// Moonshot Kimi supports json_object response format.
	if strings.Contains(u, "moonshot") || strings.Contains(u, "moonshot.cn") || strings.Contains(m, "kimi") {
		return map[string]any{"type": "json_object"}
	}
	return nil
}

var _ client.ILLMProvider = (*OpenAIProvider)(nil)
