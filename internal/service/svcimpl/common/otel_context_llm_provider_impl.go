package common

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/tiersum/tiersum/internal/client"
)

// OTelContextLLM wraps an LLM provider and records one span per Generate
// when a progressive debug tracer is present on context (or when a sampled trace requests it).
type OTelContextLLM struct {
	inner client.ILLMProvider
}

// NewOTelContextLLM wraps the process-wide LLM client for optional OpenTelemetry spans around Generate.
func NewOTelContextLLM(inner client.ILLMProvider) client.ILLMProvider {
	if inner == nil {
		return nil
	}
	return &OTelContextLLM{inner: inner}
}

func (p *OTelContextLLM) Generate(ctx context.Context, prompt string, maxTokens int) (string, error) {
	t := ProgressiveDebugTracerFrom(ctx)
	// If the query path installed a progressive tracer, use it.
	// Otherwise, fall back to global tracer when the request is part of a recording trace
	// so Generate spans still show up in persisted debug traces.
	if t == nil && ProgressiveTraceRequested(ctx) {
		t = otel.Tracer(ProgressiveTracerScope)
	}
	if t == nil {
		return p.inner.Generate(ctx, prompt, maxTokens)
	}
	ctx, sp := t.Start(ctx, "llm.Generate")
	defer sp.End()
	sp.SetAttributes(
		attribute.Int("tier.llm.request.max_tokens", maxTokens),
		attribute.String("tier.llm.request.prompt", TruncateTraceStr(prompt, TraceMaxReqBytes)),
	)
	out, err := p.inner.Generate(ctx, prompt, maxTokens)
	if err != nil {
		sp.RecordError(err)
		sp.SetStatus(codes.Error, err.Error())
		return "", err
	}
	sp.SetAttributes(attribute.String("tier.llm.response.completion", TruncateTraceStr(out, TraceMaxRespBytes)))
	return out, nil
}

var _ client.ILLMProvider = (*OTelContextLLM)(nil)
