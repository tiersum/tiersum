// Package llm implements third-party client layer
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/tiersum/tiersum/internal/client"
)

// AnthropicProvider implements client.ILLMProvider for Anthropic Claude
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewAnthropicProvider creates a new Anthropic Claude provider
func NewAnthropicProvider() *AnthropicProvider {
	timeout := viper.GetDuration("llm.anthropic.timeout")
	return &AnthropicProvider{
		apiKey:  viper.GetString("llm.anthropic.api_key"),
		baseURL: viper.GetString("llm.anthropic.base_url"),
		model:   viper.GetString("llm.anthropic.model"),
		client:  &http.Client{Timeout: timeout},
	}
}

type anthropicRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	System      string    `json:"system,omitempty"`
	Messages    []anthMsg `json:"messages"`
}

type anthMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

// Generate implements ILLMProvider.Generate
func (p *AnthropicProvider) Generate(ctx context.Context, messages []client.LLMMessage, maxTokens int) (string, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/client/llm")
	ctx, span := tr.Start(ctx, "AnthropicProvider.Generate", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(attribute.String("model", p.model))
	span.SetAttributes(attribute.Int("max_tokens", maxTokens))
	span.SetAttributes(attribute.Int("messages", len(messages)))

	temp := viper.GetFloat64("llm.anthropic.temperature")
	if temp == 0 {
		temp = 0.3
	}

	var systemPrompt string
	var msgs []anthMsg
	for i, m := range messages {
		if m.Role == client.LLMMessageRoleSystem {
			if i == 0 {
				// Anthropic supports a top-level system field; use it for the first system message.
				systemPrompt = m.Content
				continue
			}
		}
		msgs = append(msgs, anthMsg{Role: string(m.Role), Content: m.Content})
	}

	reqBody := anthropicRequest{
		Model:       p.model,
		MaxTokens:   maxTokens,
		Temperature: temp,
		System:      systemPrompt,
		Messages:    msgs,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(resp.Body)
		err := fmt.Errorf("api error: status=%d, body=%s", resp.StatusCode, errBody.String())
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	if result.Usage != nil {
		span.SetAttributes(
			attribute.Int("llm_prompt_tokens", result.Usage.InputTokens),
			attribute.Int("llm_completion_tokens", result.Usage.OutputTokens),
			attribute.Int("llm_total_tokens", result.Usage.InputTokens+result.Usage.OutputTokens),
		)
	}

	if len(result.Content) > 0 {
		return strings.TrimSpace(result.Content[0].Text), nil
	}

	err = fmt.Errorf("no response from Anthropic")
	span.SetAttributes(attribute.String("error_message", err.Error()))
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return "", err
}

var _ client.ILLMProvider = (*AnthropicProvider)(nil)
