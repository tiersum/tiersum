package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

// ColdIndexRefreshJob periodically refreshes the in-memory cold index from the shared chapters table.
// In multi-instance deployments, each instance runs this job so all converge on the same cold data.
type ColdIndexRefreshJob struct {
	maintenance service.IDocumentMaintenanceService
	logger      *zap.Logger
}

// NewColdIndexRefreshJob creates a cold index refresh job.
func NewColdIndexRefreshJob(
	maintenance service.IDocumentMaintenanceService,
	logger *zap.Logger,
) *ColdIndexRefreshJob {
	return &ColdIndexRefreshJob{
		maintenance: maintenance,
		logger:      logger,
	}
}

func (j *ColdIndexRefreshJob) Name() string {
	return "cold_index_refresh"
}

func (j *ColdIndexRefreshJob) Interval() time.Duration {
	return 30 * time.Second
}

func (j *ColdIndexRefreshJob) Execute(ctx context.Context) error {
	if err := j.maintenance.RefreshColdIndex(ctx); err != nil {
		j.logger.Error("cold index refresh job failed", zap.Error(err))
		return err
	}
	return nil
}

var _ Job = (*ColdIndexRefreshJob)(nil)
