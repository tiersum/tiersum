package job

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// StartChannelConsumer invokes handle for each item from ch until ctx is cancelled.
// Each item runs under a child context with the given timeout (relative to ctx).
func StartChannelConsumer[T any](
	ctx context.Context,
	logger *zap.Logger,
	consumerName string,
	ch <-chan T,
	timeout time.Duration,
	handle func(context.Context, T) error,
	onErr func(*zap.Logger, T, error),
) {
	if logger == nil {
		return
	}
	if onErr == nil {
		onErr = func(l *zap.Logger, item T, err error) {
			l.Error(consumerName, zap.Error(err))
		}
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case item := <-ch:
				runCtx, cancel := context.WithTimeout(ctx, timeout)
				if telemetry.GlobalTracerActive() {
					tr := otel.Tracer("github.com/tiersum/tiersum/job")
					runCtx, span := tr.Start(runCtx, "queue."+consumerName, trace.WithSpanKind(trace.SpanKindInternal))
					span.SetAttributes(attribute.String("queue", consumerName))
					err := handle(runCtx, item)
					if err != nil {
						span.RecordError(err)
						span.SetAttributes(attribute.Bool("error", true))
					}
					span.End()
					cancel()
					if err != nil {
						onErr(logger, item, err)
					}
				} else {
					err := handle(runCtx, item)
					cancel()
					if err != nil {
						onErr(logger, item, err)
					}
				}
			}
		}
	}()
	logger.Info(consumerName + " consumer started")
}
