package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

// TagGroupJob periodically performs tag grouping
type TagGroupJob struct {
	groupingSvc service.ITagGroupService
	logger      *zap.Logger
}

// NewTagGroupJob creates a new tag grouping job
func NewTagGroupJob(
	groupingSvc service.ITagGroupService,
	logger *zap.Logger,
) *TagGroupJob {
	return &TagGroupJob{
		groupingSvc: groupingSvc,
		logger:      logger,
	}
}

// Name returns the job name
func (j *TagGroupJob) Name() string {
	return "tag_grouping"
}

// Interval returns the execution interval (30 minutes)
func (j *TagGroupJob) Interval() time.Duration {
	return 30 * time.Minute
}

// Execute performs the tag grouping job
// Strategy:
// 1. Check if grouping is needed (tag count changed or time elapsed)
// 2. If needed, perform LLM-based grouping
// 3. Update Level 1 groups and Level 2 tag assignments
func (j *TagGroupJob) Execute(ctx context.Context) error {
	j.logger.Info("running tag grouping job")

	// Check if refresh is needed
	shouldRefresh, err := j.groupingSvc.ShouldRefresh(ctx)
	if err != nil {
		j.logger.Error("failed to check if refresh needed", zap.Error(err))
		return err
	}

	if !shouldRefresh {
		j.logger.Info("tag grouping not needed at this time")
		return nil
	}

	// Perform grouping
	if err := j.groupingSvc.GroupTags(ctx); err != nil {
		j.logger.Error("failed to group tags", zap.Error(err))
		return err
	}

	j.logger.Info("tag grouping completed successfully")
	return nil
}
