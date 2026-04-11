package job

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/internal/service"
)

// HotScoreJob periodically updates hot scores for all documents.
type HotScoreJob struct {
	maintenance service.IDocumentMaintenanceService
}

// NewHotScoreJob creates a new hot score calculation job.
func NewHotScoreJob(maintenance service.IDocumentMaintenanceService) *HotScoreJob {
	return &HotScoreJob{maintenance: maintenance}
}

// Name returns the job name
func (j *HotScoreJob) Name() string {
	return "hot_score_update"
}

// Interval returns the execution interval (runs every hour)
func (j *HotScoreJob) Interval() time.Duration {
	return 1 * time.Hour
}

// Execute runs the hot score recalculation pass.
func (j *HotScoreJob) Execute(ctx context.Context) error {
	return j.maintenance.RecalculateDocumentHotScores(ctx)
}
