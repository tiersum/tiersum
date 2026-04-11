package job

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/internal/service"
)

// PromoteJob handles async promotion of cold documents to hot status.
type PromoteJob struct {
	maintenance service.IDocumentMaintenanceService
}

// NewPromoteJob creates a new document promotion job.
func NewPromoteJob(maintenance service.IDocumentMaintenanceService) *PromoteJob {
	return &PromoteJob{maintenance: maintenance}
}

// Name returns the job name
func (j *PromoteJob) Name() string {
	return "document_promote"
}

// Interval returns the execution interval
func (j *PromoteJob) Interval() time.Duration {
	return 5 * time.Minute
}

// Execute runs the scheduled cold-document promotion sweep.
func (j *PromoteJob) Execute(ctx context.Context) error {
	return j.maintenance.RunColdPromotionSweep(ctx)
}
