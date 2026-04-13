package svcimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tiersum/tiersum/pkg/metrics"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// TopicSvc implements service.ITopicService (LLM regrouping of catalog tags into topics).
type TopicSvc struct {
	tagRepo         storage.ITagRepository
	topicRepo       storage.ITopicRepository
	provider        client.ILLMProvider
	logger          *zap.Logger
	lastRefreshTime time.Time
	lastTagCount    int
}

// NewTopicSvc creates a new topic / regrouping service.
func NewTopicSvc(
	tagRepo storage.ITagRepository,
	topicRepo storage.ITopicRepository,
	provider client.ILLMProvider,
	logger *zap.Logger,
) *TopicSvc {
	return &TopicSvc{
		tagRepo:         tagRepo,
		topicRepo:       topicRepo,
		provider:        provider,
		logger:          logger,
		lastRefreshTime: time.Time{},
		lastTagCount:    0,
	}
}

// RegroupTags assigns every catalog tag to exactly one topic using the LLM.
func (s *TopicSvc) RegroupTags(ctx context.Context) error {
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

// ShouldRefresh checks if regrouping should run (tag count change or interval).
func (s *TopicSvc) ShouldRefresh(ctx context.Context) (bool, error) {
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

// ListTopics returns all topics.
func (s *TopicSvc) ListTopics(ctx context.Context) ([]types.Topic, error) {
	return s.topicRepo.List(ctx)
}

// ListTagsByTopic returns catalog tags for one topic.
func (s *TopicSvc) ListTagsByTopic(ctx context.Context, topicID string) ([]types.Tag, error) {
	return s.tagRepo.ListByTopic(ctx, topicID)
}

// FilterTagsByQuery uses the LLM to score catalog tags for a query.
func (s *TopicSvc) FilterTagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	var tagList strings.Builder
	for _, tag := range tags {
		tagList.WriteString(fmt.Sprintf("- %s (used in %d documents)\n", tag.Name, tag.DocumentCount))
	}

	prompt := fmt.Sprintf(`Given the user query: "%s"

Select the most relevant tags from the list below. Return a JSON array of objects with fields "tag" and "relevance" (0.0-1.0 score).
Only include tags with relevance >= 0.6. Sort by relevance descending.

Available tags:
%s

Response format (JSON only):
[
  {"tag": "tag-name", "relevance": 0.95},
  {"tag": "another-tag", "relevance": 0.82}
]`, query, tagList.String())

	metrics.RecordLLMCall(metrics.PathTagFilter, estimateTokens(prompt))

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		s.logger.Error("LLM tag filter failed", zap.Error(err))
		return s.fallbackTagFilter(tags), nil
	}

	return s.parseTagFilterResults(response), nil
}

func (s *TopicSvc) performGrouping(_ context.Context, tags []string) ([]types.Topic, error) {
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

	metrics.RecordLLMCall(metrics.PathTopicRegroup, estimateTokens(prompt))

	response, err := s.provider.Generate(ctx, prompt, 3000)
	if err != nil {
		return nil, fmt.Errorf("LLM grouping failed: %w", err)
	}

	return s.parseGroupResponse(response)
}

func (s *TopicSvc) parseGroupResponse(response string) ([]types.Topic, error) {
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

func (s *TopicSvc) parseTagFilterResults(response string) []types.TagFilterResult {
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var results []types.TagFilterResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		s.logger.Warn("failed to parse tag filter results", zap.Error(err))
		return nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})

	return results
}

func (s *TopicSvc) fallbackTagFilter(tags []types.Tag) []types.TagFilterResult {
	results := make([]types.TagFilterResult, len(tags))
	for i, tag := range tags {
		results[i] = types.TagFilterResult{
			Tag:       tag.Name,
			Relevance: 0.5,
		}
	}
	return results
}

var _ service.ITopicService = (*TopicSvc)(nil)
