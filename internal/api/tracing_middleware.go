package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/tiersum/tiersum/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// httpSpanStatusFromResponseCode maps the HTTP response status written by the handler to an
// OpenTelemetry span status, following common APM practice and OpenTelemetry HTTP semantic
// conventions: 1xx–3xx are successful handling; 4xx (client errors) and 5xx (server errors)
// mark the span as Error so trace UIs and SLO/error budgets align with failed requests.
// The numeric code remains on the span as attribute http.status_code.
func httpSpanStatusFromResponseCode(code int) (codes.Code, string) {
	if code <= 0 {
		return codes.Ok, ""
	}
	if code >= http.StatusBadRequest {
		text := http.StatusText(code)
		if text == "" {
			return codes.Error, fmt.Sprintf("HTTP %d", code)
		}
		return codes.Error, fmt.Sprintf("HTTP %d %s", code, text)
	}
	return codes.Ok, ""
}

// NewTracingMiddleware records one server span per request for core TierSum APIs.
// Forced sampling uses telemetry.force_sample_query_param and telemetry.force_sample_header (truthy values).
func NewTracingMiddleware() gin.HandlerFunc {
	propagator := otel.GetTextMapPropagator()
	tr := otel.Tracer("github.com/tiersum/tiersum/api")

	return func(c *gin.Context) {
		carrier := propagation.HeaderCarrier(c.Request.Header)
		ctx := propagator.Extract(c.Request.Context(), carrier)

		force := telemetryForceFromRequest(c)
		ctx = telemetry.ContextWithForceSample(ctx, force)

		spanName := c.Request.Method + " " + c.Request.URL.Path
		ctx, span := tr.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		c.Request = c.Request.WithContext(ctx)

		c.Next()

		if route := c.FullPath(); route != "" {
			span.SetName(c.Request.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route))
		}
		span.SetAttributes(attribute.String("http.method", c.Request.Method))
		code := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.status_code", code))
		st, msg := httpSpanStatusFromResponseCode(code)
		span.SetStatus(st, msg)
	}
}

func telemetryForceFromRequest(c *gin.Context) bool {
	qn := strings.TrimSpace(viper.GetString("telemetry.force_sample_query_param"))
	hn := strings.TrimSpace(viper.GetString("telemetry.force_sample_header"))
	if telemetry.TruthyQuery(c, qn) {
		return true
	}
	if telemetry.TruthyHeader(c, hn) {
		return true
	}
	return false
}
