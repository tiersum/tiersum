package shared

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// SetSpanStatus records the OpenTelemetry span status.
// On error it sets codes.Error with the message and records the error.
// On success it sets codes.Ok.
func SetSpanStatus(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// SetSpanInputID records a single input entity ID.
func SetSpanInputID(span trace.Span, id string) {
	if span == nil || id == "" {
		return
	}
	span.SetAttributes(attribute.String("input.id", id))
}

// SetSpanInputIDs records a list of input entity IDs.
func SetSpanInputIDs(span trace.Span, ids []string) {
	if span == nil || len(ids) == 0 {
		return
	}
	span.SetAttributes(attribute.StringSlice("input.ids", ids))
}

// SetSpanOutputCount records the number of returned entities.
func SetSpanOutputCount(span trace.Span, n int) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.Int("output.count", n))
}

// SetSpanOutputIDs records a list of output entity IDs (truncated to max 50 to avoid span bloat).
func SetSpanOutputIDs(span trace.Span, ids []string) {
	if span == nil || len(ids) == 0 {
		return
	}
	const maxIDs = 50
	if len(ids) > maxIDs {
		ids = ids[:maxIDs]
	}
	span.SetAttributes(attribute.StringSlice("output.ids", ids))
}

// SetSpanOutputID records a single output entity ID.
func SetSpanOutputID(span trace.Span, id string) {
	if span == nil || id == "" {
		return
	}
	span.SetAttributes(attribute.String("output.id", id))
}

// SetSpanInputString records a generic string input parameter.
func SetSpanInputString(span trace.Span, key, value string) {
	if span == nil || value == "" {
		return
	}
	span.SetAttributes(attribute.String("input."+key, value))
}

// CollectIDs extracts the ID field from a slice of items using the provided getter.
func CollectIDs[T any](items []T, getID func(T) string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		id := getID(item)
		if strings.TrimSpace(id) != "" {
			out = append(out, id)
		}
	}
	return out
}
