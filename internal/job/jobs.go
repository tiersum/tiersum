package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

// TopicRegroupJob periodically regroups catalog tags into topics when ShouldRefresh is true.
type TopicRegroupJob struct {
	topicService service.ITopicService
	logger       *zap.Logger
}

// NewTopicRegroupJob creates a topic regroup job.
func NewTopicRegroupJob(
	topicService service.ITopicService,
	logger *zap.Logger,
) *TopicRegroupJob {
	return &TopicRegroupJob{
		topicService: topicService,
		logger:       logger,
	}
}

func (j *TopicRegroupJob) Name() string {
	return "topic_regroup"
}

func (j *TopicRegroupJob) Interval() time.Duration {
	return 30 * time.Minute
}

func (j *TopicRegroupJob) Execute(ctx context.Context) error {
	shouldRefresh, err := j.topicService.ShouldRefresh(ctx)
	if err != nil {
		j.logger.Error("failed to check topic regroup refresh", zap.Error(err))
		return err
	}

	if !shouldRefresh {
		j.logger.Debug("topic regroup not needed")
		return nil
	}

	j.logger.Info("running scheduled topic regroup")
	if err := j.topicService.RegroupTags(ctx); err != nil {
		j.logger.Error("topic regroup failed", zap.Error(err))
		return err
	}

	j.logger.Info("topic regroup completed successfully")
	return nil
}
