package job

import (
	"context"
	"time"

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
				err := handle(runCtx, item)
				cancel()
				if err != nil {
					onErr(logger, item, err)
				}
			}
		}
	}()
	logger.Info(consumerName + " consumer started")
}
