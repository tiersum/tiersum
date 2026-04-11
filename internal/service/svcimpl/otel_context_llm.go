package svcimpl

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tiersum/tiersum/internal/client"
)

// otelContextLLM wraps an LLM provider and records one span per Generate when a progressive debug tracer is on context.
type otelContextLLM struct {
	inner client.ILLMProvider
}

// NewOTelContextLLM wraps the shared LLM for optional progressive-query tracing.
func NewOTelContextLLM(inner client.ILLMProvider) client.ILLMProvider {
	if inner == nil {
		return nil
	}
	return &otelContextLLM{inner: inner}
}

func (p *otelContextLLM) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	t := ProgressiveDebugTracerFrom(ctx)
	if t == nil {
		return p.inner.Generate(ctx, prompt, maxTokens)
	}
	ctx, sp := t.Start(ctx, "llm.Generate")
	defer sp.End()
	sp.SetAttributes(
		attribute.Int("tier.llm.request.max_tokens", maxTokens),
		attribute.String("tier.llm.request.prompt", truncateTraceStr(prompt, traceMaxReqBytes)),
	)
	out, err := p.inner.Generate(ctx, prompt, maxTokens)
	if err != nil {
		sp.RecordError(err)
		sp.SetStatus(codes.Error, err.Error())
		return "", err
	}
	sp.SetAttributes(attribute.String("tier.llm.response.completion", truncateTraceStr(out, traceMaxRespBytes)))
	return out, nil
}

var _ client.ILLMProvider = (*otelContextLLM)(nil)
