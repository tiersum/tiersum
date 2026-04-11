package telemetry

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// ContextFromTraceparent merges W3C Trace Context from a traceparent value (same format as the
// traceparent HTTP header) into ctx. Use this when the client cannot attach HTTP headers (e.g. MCP JSON tools).
func ContextFromTraceparent(ctx context.Context, traceparent string) context.Context {
	tp := strings.TrimSpace(traceparent)
	if tp == "" {
		return ctx
	}
	h := http.Header{}
	h.Set("traceparent", tp)
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(h))
}
