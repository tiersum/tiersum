package job

import (
	"context"
	"time"

	"github.com/tiersum/tiersum/internal/service"
)

// maintenanceDelegateJob runs a named maintenance sweep on a fixed interval.
type maintenanceDelegateJob struct {
	name           string
	interval       time.Duration
	maintenance    service.IDocumentMaintenanceService
	runMaintenance func(ctx context.Context, m service.IDocumentMaintenanceService) error
}

func (j *maintenanceDelegateJob) Name() string {
	return j.name
}

func (j *maintenanceDelegateJob) Interval() time.Duration {
	return j.interval
}

func (j *maintenanceDelegateJob) Execute(ctx context.Context) error {
	return j.runMaintenance(ctx, j.maintenance)
}

// NewPromoteJob schedules cold-document promotion sweeps.
func NewPromoteJob(m service.IDocumentMaintenanceService) Job {
	return &maintenanceDelegateJob{
		name:        "document_promote",
		interval:    5 * time.Minute,
		maintenance: m,
		runMaintenance: func(ctx context.Context, x service.IDocumentMaintenanceService) error {
			return x.RunColdPromotionSweep(ctx)
		},
	}
}

// NewHotScoreJob schedules hot-score recalculation.
func NewHotScoreJob(m service.IDocumentMaintenanceService) Job {
	return &maintenanceDelegateJob{
		name:        "hot_score_update",
		interval:    1 * time.Hour,
		maintenance: m,
		runMaintenance: func(ctx context.Context, x service.IDocumentMaintenanceService) error {
			return x.RecalculateDocumentHotScores(ctx)
		},
	}
}

var _ Job = (*maintenanceDelegateJob)(nil)
