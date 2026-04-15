package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tiersum/tiersum/pkg/metrics"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl/common"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewTopicService constructs the service.ITopicService implementation.
func NewTopicService(
	tagRepo storage.ITagRepository,
	topicRepo storage.ITopicRepository,
	provider client.ILLMProvider,
	logger *zap.Logger,
) service.ITopicService {
	return &topicService{
		tagRepo:         tagRepo,
		topicRepo:       topicRepo,
		provider:        provider,
		logger:          logger,
		lastRefreshTime: time.Time{},
		lastTagCount:    0,
	}
}

type topicService struct {
	tagRepo         storage.ITagRepository
	topicRepo       storage.ITopicRepository
	provider        client.ILLMProvider
	logger          *zap.Logger
	lastRefreshTime time.Time
	lastTagCount    int
}

func (s *topicService) RegroupTags(ctx context.Context) error {
	startTime := time.Now()

	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list catalog tags: %w", err)
	}

	tagCountBefore := len(tags)
	if tagCountBefore == 0 {
		s.logger.Info("no tags to regroup")
		return nil
	}

	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagNames[i] = tag.Name
	}

	topics, err := s.performGrouping(ctx, tagNames)
	if err != nil {
		return fmt.Errorf("perform grouping: %w", err)
	}

	if err := s.topicRepo.DeleteAll(ctx); err != nil {
		return fmt.Errorf("delete existing topics: %w", err)
	}

	for _, topic := range topics {
		if err := s.topicRepo.Create(ctx, &topic); err != nil {
			s.logger.Warn("failed to create topic", zap.String("name", topic.Name), zap.Error(err))
			continue
		}

		for _, tagName := range topic.TagNames {
			tag, err := s.tagRepo.GetByName(ctx, tagName)
			if err != nil {
				s.logger.Warn("failed to get tag", zap.String("name", tagName), zap.Error(err))
				continue
			}
			if tag == nil {
				continue
			}

			tag.TopicID = topic.ID
			if err := s.tagRepo.Create(ctx, tag); err != nil {
				s.logger.Warn("failed to update tag topic", zap.String("tag", tagName), zap.Error(err))
			}
		}
	}

	duration := time.Since(startTime).Milliseconds()

	s.lastRefreshTime = time.Now()
	s.lastTagCount = tagCountBefore

	s.logger.Info("completed topic regrouping",
		zap.Int("tags", tagCountBefore),
		zap.Int("topics", len(topics)),
		zap.Int64("duration_ms", duration))

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

func (s *topicService) performGrouping(ctx context.Context, tags []string) ([]types.Topic, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	tagList := strings.Join(tags, "\n")

	targetTopics := len(tags) / 10
	if targetTopics < 3 {
		targetTopics = 3
	}
	if targetTopics > 10 {
		targetTopics = 10
	}

	prompt := fmt.Sprintf(`Group the following tags into %d topics (themes). Each topic should have a clear theme and contain related tags.
Aim for balanced distribution (each topic should have roughly similar number of tags).

Tags to group:
%s

Return a JSON array where each element has:
- "name": topic name (2-4 words)
- "description": brief description (max 100 chars)
- "tags": array of tag names belonging to this topic

Response format (JSON only):
[
  {
    "name": "Topic Name",
    "description": "Description of this topic",
    "tags": ["tag1", "tag2", ...]
  },
  ...
]

Make sure every tag appears in exactly one topic.`, targetTopics, tagList)

	metrics.RecordLLMCall(metrics.PathTopicRegroup, common.EstimateTokens(prompt))

	response, err := s.provider.Generate(ctx, prompt, 3000)
	if err != nil {
		return nil, fmt.Errorf("LLM grouping failed: %w", err)
	}

	return s.parseGroupResponse(response)
}

func (s *topicService) parseGroupResponse(response string) ([]types.Topic, error) {
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var rawGroups []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawGroups); err != nil {
		return nil, fmt.Errorf("failed to parse topic JSON: %w", err)
	}

	topics := make([]types.Topic, len(rawGroups))
	for i, rg := range rawGroups {
		topics[i] = types.Topic{
			Name:        rg.Name,
			Description: rg.Description,
			TagNames:    rg.Tags,
		}
	}

	return topics, nil
}

var _ service.ITopicService = (*topicService)(nil)
