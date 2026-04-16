package di

import (
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
	"go.uber.org/zap"
)

// hotIngestQueueSink forwards work to job.HotIngestQueue without blocking the HTTP handler.
type hotIngestQueueSink struct {
	logger *zap.Logger
}

// NewHotIngestQueueSink constructs a service.IHotIngestWorkSink backed by the process-local hot ingest channel.
func NewHotIngestQueueSink(logger *zap.Logger) service.IHotIngestWorkSink {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &hotIngestQueueSink{logger: logger}
}

func (s *hotIngestQueueSink) SubmitHotIngest(work types.HotIngestWork) {
	if work.DocID == "" {
		return
	}
	select {
	case job.HotIngestQueue <- work:
	default:
		s.logger.Warn("hot ingest queue full; deferred materialization skipped",
			zap.String("doc_id", work.DocID))
	}
}

var _ service.IHotIngestWorkSink = (*hotIngestQueueSink)(nil)
