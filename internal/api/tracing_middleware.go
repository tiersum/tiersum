package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/tiersum/tiersum/internal/telemetry"
)

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
		if code >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(code))
		}
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
