package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

// StartHotIngestQueueConsumer reads HotIngestQueue and invokes IHotIngestProcessor.ProcessHotIngest (LLM + parse, then persist; failures become a virtual failure chapter when possible).
// It runs until ctx is cancelled.
func StartHotIngestQueueConsumer(ctx context.Context, proc service.IHotIngestProcessor, logger *zap.Logger) {
	if proc == nil || logger == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case work := <-HotIngestQueue:
				runCtx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
				err := proc.ProcessHotIngest(runCtx, work)
				cancel()
				if err != nil {
					logger.Error("hot ingest queue: processing failed",
						zap.String("doc_id", work.DocID),
						zap.Error(err))
				}
			}
		}
	}()
	logger.Info("hot ingest queue consumer started")
}
