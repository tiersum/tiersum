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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/tiersum/tiersum/internal/client"
)

// OllamaProvider implements client.ILLMProvider for local Ollama models using the /api/chat endpoint.
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

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  struct {
		NumPredict  int     `json:"num_predict,omitempty"`
		Temperature float64 `json:"temperature,omitempty"`
	} `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
	// Ollama may return these counters on newer versions.
	PromptEvalCount int `json:"prompt_eval_count,omitempty"`
	EvalCount       int `json:"eval_count,omitempty"`
}

// Generate implements ILLMProvider.Generate using Ollama's /api/chat endpoint.
func (p *OllamaProvider) Generate(ctx context.Context, messages []client.LLMMessage, maxTokens int) (string, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/client/llm")
	ctx, span := tr.Start(ctx, "OllamaProvider.Generate", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(attribute.String("model", p.model))
	span.SetAttributes(attribute.Int("max_tokens", maxTokens))
	span.SetAttributes(attribute.Int("messages", len(messages)))

	msgs := make([]ollamaChatMessage, len(messages))
	for i, m := range messages {
		msgs[i] = ollamaChatMessage{Role: string(m.Role), Content: m.Content}
	}

	reqBody := ollamaChatRequest{
		Model:    p.model,
		Messages: msgs,
		Stream:   false,
	}
	reqBody.Options.NumPredict = maxTokens
	reqBody.Options.Temperature = 0.3

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

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
		err := fmt.Errorf("ollama api error: status=%d, body=%s", resp.StatusCode, errBody.String())
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	var result ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		span.SetAttributes(attribute.String("error_message", err.Error()))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	if result.PromptEvalCount > 0 && result.EvalCount > 0 {
		span.SetAttributes(
			attribute.Int("llm_prompt_tokens", result.PromptEvalCount),
			attribute.Int("llm_completion_tokens", result.EvalCount),
			attribute.Int("llm_total_tokens", result.PromptEvalCount+result.EvalCount),
		)
	} else {
		inputText := ""
		for _, m := range messages {
			inputText += m.Content
		}
		span.SetAttributes(
			attribute.Int("llm_estimated_prompt_tokens", roughTokenCount(inputText)),
			attribute.Int("llm_estimated_completion_tokens", roughTokenCount(result.Message.Content)),
		)
	}

	if result.Done {
		return result.Message.Content, nil
	}

	err = fmt.Errorf("no response from Ollama")
	span.SetAttributes(attribute.String("error_message", err.Error()))
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return "", err
}

func roughTokenCount(text string) int {
	if text == "" {
		return 0
	}
	cn := 0
	for _, r := range text {
		if r > 127 {
			cn++
		}
	}
	return cn + (len(text)-cn)/4
}

var _ client.ILLMProvider = (*OllamaProvider)(nil)
