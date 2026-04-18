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
	StartChannelConsumer(ctx, logger, "promote queue", PromoteQueue, 12*time.Minute,
		func(runCtx context.Context, docID string) error {
			return maintenance.PromoteColdDocumentByID(runCtx, docID)
		},
		func(l *zap.Logger, docID string, err error) {
			l.Error("promote queue: promotion failed",
				zap.String("doc_id", docID),
				zap.Error(err))
		},
	)
}
