package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// StartHotIngestQueueConsumer reads HotIngestQueue and invokes IHotIngestProcessor.ProcessHotIngest.
// It runs until ctx is cancelled.
func StartHotIngestQueueConsumer(ctx context.Context, proc service.IHotIngestProcessor, logger *zap.Logger) {
	if proc == nil || logger == nil {
		return
	}
	StartChannelConsumer(ctx, logger, "hot ingest queue", HotIngestQueue, 12*time.Minute,
		func(runCtx context.Context, work types.HotIngestWork) error {
			return proc.ProcessHotIngest(runCtx, work)
		},
		func(l *zap.Logger, work types.HotIngestWork, err error) {
			l.Error("hot ingest queue: processing failed",
				zap.String("doc_id", work.DocID),
				zap.Error(err))
		},
	)
}
