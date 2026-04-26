package shared

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tiersum/tiersum/internal/telemetry"
)

// WithRepoSpan starts a storage-layer span when tracing is active.
// Returns the original context and a nil span when tracing is disabled.
func WithRepoSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if !telemetry.GlobalTracerActive() {
		return ctx, nil
	}
	tr := otel.Tracer("github.com/tiersum/tiersum/storage/db")
	ctx, span := tr.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}
