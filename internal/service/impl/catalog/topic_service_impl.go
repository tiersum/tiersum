package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewTopicService constructs a lightweight service.ITopicService implementation.
//
// This rewrite-phase implementation supports:
// - ListTopics (topic browser UI)
// - ShouldRefresh (jobs/UI hints)
// - RegroupTags (deterministic, non-LLM grouping) so the UI button has an effect
func NewTopicService(tagRepo storage.ITagRepository, topicRepo storage.ITopicRepository) service.ITopicService {
	return &topicService{
		tagRepo:         tagRepo,
		topicRepo:       topicRepo,
		lastRefreshTime: time.Time{},
		lastTagCount:    0,
	}
}

type topicService struct {
	tagRepo         storage.ITagRepository
	topicRepo       storage.ITopicRepository
	lastRefreshTime time.Time
	lastTagCount    int
}

func (s *topicService) RegroupTags(ctx context.Context) error {
	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list catalog tags: %w", err)
	}
	if len(tags) == 0 {
		s.lastRefreshTime = time.Now()
		s.lastTagCount = 0
		return nil
	}

	// Deterministic regroup during rewrite: one topic containing all current tags.
	topic := &types.Topic{
		Name:        "All tags",
		Description: "Deterministic grouping (rewrite phase)",
		TagNames:    make([]string, 0, len(tags)),
	}
	for _, t := range tags {
		topic.TagNames = append(topic.TagNames, t.Name)
	}

	if err := s.topicRepo.DeleteAll(ctx); err != nil {
		return fmt.Errorf("delete existing topics: %w", err)
	}
	if err := s.topicRepo.Create(ctx, topic); err != nil {
		return fmt.Errorf("create topic: %w", err)
	}

	for _, t := range tags {
		tt := t
		tt.TopicID = topic.ID
		if err := s.tagRepo.Create(ctx, &tt); err != nil {
			return fmt.Errorf("update tag topic: %w", err)
		}
	}

	s.lastRefreshTime = time.Now()
	s.lastTagCount = len(tags)
	return nil
}

func (s *topicService) ShouldRefresh(ctx context.Context) (bool, error) {
	currentCount, err := s.tagRepo.GetCount(ctx)
	if err != nil {
		return false, fmt.Errorf("get tag count: %w", err)
	}
	if s.lastRefreshTime.IsZero() {
		return true, nil
	}
	if currentCount != s.lastTagCount {
		return true, nil
	}
	if time.Since(s.lastRefreshTime) > 30*time.Minute {
		return true, nil
	}
	return false, nil
}

func (s *topicService) ListTopics(ctx context.Context) ([]types.Topic, error) {
	return s.topicRepo.List(ctx)
}

var _ service.ITopicService = (*topicService)(nil)

