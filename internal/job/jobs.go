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
	if err := j.topicService.RegroupTagsIfNeeded(ctx); err != nil {
		j.logger.Error("topic regroup job failed", zap.Error(err))
		return err
	}
	j.logger.Debug("topic regroup job finished")
	return nil
}
