package svcimpl

import (
	"context"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	viper.SetDefault("query.allow_progressive_debug", true)
}

const (
	traceMaxReqBytes  = 4096
	traceMaxRespBytes = 4096
)

// Progressive-query span attributes use a consistent prefix:
//   - tier.request.*  inputs (question, limits, prompts)
//   - tier.response.* outputs (counts, hits, model text, flags)

type debugTracerKeyType struct{}

var debugTracerKey = debugTracerKeyType{}

const progressiveTracerScope = "github.com/tiersum/tiersum/progressive_query"

// WithProgressiveDebugTracer attaches a request-local OpenTelemetry tracer for progressive-query debug recording.
func WithProgressiveDebugTracer(ctx context.Context, t trace.Tracer) context.Context {
	if t == nil {
		return ctx
	}
	return context.WithValue(ctx, debugTracerKey, t)
}

// ProgressiveDebugTracerFrom returns the tracer installed by WithProgressiveDebugTracer, or nil.
func ProgressiveDebugTracerFrom(ctx context.Context) trace.Tracer {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(debugTracerKey).(trace.Tracer)
	return v
}

// progressiveTraceRequested is true when the server allows detailed progressive spans and the
// request is part of a sampled trace (parent span in ctx is recording), per OpenTelemetry practice.
func progressiveTraceRequested(ctx context.Context) bool {
	if !viper.GetBool("query.allow_progressive_debug") {
		return false
	}
	s := trace.SpanFromContext(ctx)
	return s.SpanContext().IsValid() && s.IsRecording()
}

// withOptionalSpan runs fn with an active child span when ProgressiveDebugTracerFrom is non-nil.
// If sp is non-nil, fn should record errors on sp; the wrapper still ends the span after fn returns.
func withOptionalSpan(ctx context.Context, name string, fn func(context.Context, trace.Span) error) error {
	tr := ProgressiveDebugTracerFrom(ctx)
	if tr == nil {
		return fn(ctx, nil)
	}
	ctx2, sp := tr.Start(ctx, name)
	defer sp.End()
	err := fn(ctx2, sp)
	if err != nil {
		sp.RecordError(err)
		sp.SetStatus(codes.Error, err.Error())
	}
	return err
}

func truncateTraceStr(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	b := []byte(s)
	if len(b) <= maxBytes {
		return s
	}
	b = b[:maxBytes]
	for len(b) > 0 && b[len(b)-1]&0xc0 == 0x80 {
		b = b[:len(b)-1]
	}
	return string(b) + "…"
}
