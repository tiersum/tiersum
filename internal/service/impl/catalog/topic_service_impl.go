package catalog

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewTopicService constructs service.ITopicService with a deterministic regroup path (no LLM).
//
// Supported:
// - ListTopics (topic browser UI)
// - ShouldRefresh (jobs/UI hints)
// - RegroupTags — single catch-all topic over all catalog tags
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

func (s *topicService) RegroupTagsIfNeeded(ctx context.Context) error {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "RegroupTagsIfNeeded", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	shouldRefresh, err := s.ShouldRefresh(ctx)
	if err != nil {
		return err
	}
	if !shouldRefresh {
		return nil
	}
	return s.RegroupTags(ctx)
}

func (s *topicService) RegroupTags(ctx context.Context) error {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "RegroupTags", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list catalog tags: %w", err)
	}
	if len(tags) == 0 {
		s.lastRefreshTime = time.Now()
		s.lastTagCount = 0
		return nil
	}

	// Deterministic regroup: one topic containing all current catalog tags.
	topic := &types.Topic{
		Name:        "All tags",
		Description: "Deterministic grouping (all catalog tags)",
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
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "ShouldRefresh", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

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
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "ListTopics", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	return s.topicRepo.List(ctx)
}

var _ service.ITopicService = (*topicService)(nil)
