package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

// StartPromoteQueueConsumer reads document IDs from PromoteQueue and promotes cold documents
// that meet the configured query-count threshold. It runs until ctx is cancelled.
func StartPromoteQueueConsumer(ctx context.Context, maintenance service.IDocumentMaintenanceService, logger *zap.Logger) {
	if maintenance == nil || logger == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case docID := <-PromoteQueue:
				runCtx, cancel := context.WithTimeout(ctx, 12*time.Minute)
				err := maintenance.PromoteColdDocumentByID(runCtx, docID)
				cancel()
				if err != nil {
					logger.Error("promote queue: promotion failed",
						zap.String("doc_id", docID),
						zap.Error(err))
				}
			}
		}
	}()
	logger.Info("promote queue consumer started")
}
