package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
)

var globalTP *sdktrace.TracerProvider

// GlobalTracerActive reports whether InitGlobalTracer installed a real TracerProvider.
func GlobalTracerActive() bool {
	return globalTP != nil
}

// InitGlobalTracer registers a TracerProvider that exports HTTP and in-process spans to the
// application database when telemetry.enabled and telemetry.persist_to_db are true.
// It is safe to call when disabled (returns a no-op shutdown function).
func InitGlobalTracer(repo storage.IOtelSpanRepository, logger *zap.Logger) func(context.Context) error {
	if !viper.GetBool("telemetry.enabled") {
		return func(context.Context) error { return nil }
	}
	if repo == nil || !viper.GetBool("telemetry.persist_to_db") {
		if logger != nil {
			logger.Warn("telemetry.enabled is true but persist_to_db is false or span repository is nil; global HTTP tracing is not started")
		}
		return func(context.Context) error { return nil }
	}

	ratio := viper.GetFloat64("telemetry.sample_ratio")
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	exp := NewSQLSpanExporter(repo)
	bsp := sdktrace.NewBatchSpanProcessor(exp,
		sdktrace.WithBatchTimeout(2*time.Second),
		sdktrace.WithMaxExportBatchSize(256),
	)
	sampler := NewHTTPSampler(ratio)
	serviceName := viper.GetString("telemetry.service_name")
	if serviceName == "" {
		serviceName = "tiersum"
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		if logger != nil {
			logger.Warn("telemetry resource init failed, using default", zap.Error(err))
		}
		res = resource.Default()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
	)
	globalTP = tp
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	))

	if logger != nil {
		logger.Info("OpenTelemetry global tracer initialized",
			zap.Float64("telemetry.sample_ratio", ratio),
			zap.String("telemetry.service_name", serviceName),
		)
	}

	return func(ctx context.Context) error {
		globalTP = nil
		err := tp.Shutdown(ctx)
		otel.SetTracerProvider(oteltrace.NewNoopTracerProvider())
		if err != nil {
			return fmt.Errorf("tracer shutdown: %w", err)
		}
		return nil
	}
}

// FlushSpans flushes queued spans to the database (best-effort).
func FlushSpans(ctx context.Context) error {
	if globalTP == nil {
		return nil
	}
	return globalTP.ForceFlush(ctx)
}
