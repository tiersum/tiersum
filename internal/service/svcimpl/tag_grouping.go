package svcimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// TagGroupSvc implements service.ITagGroupService
type TagGroupSvc struct {
	tagRepo   storage.ITagRepository
	groupRepo storage.ITagGroupRepository
	logRepo   storage.ITagGroupRefreshLogRepository
	provider  client.ILLMProvider
	logger    *zap.Logger
}

// NewTagGroupSvc creates a new tag grouping service
func NewTagGroupSvc(
	tagRepo storage.ITagRepository,
	groupRepo storage.ITagGroupRepository,
	logRepo storage.ITagGroupRefreshLogRepository,
	provider client.ILLMProvider,
	logger *zap.Logger,
) *TagGroupSvc {
	return &TagGroupSvc{
		tagRepo:   tagRepo,
		groupRepo: groupRepo,
		logRepo:   logRepo,
		provider:  provider,
		logger:    logger,
	}
}

// GroupTags performs LLM-based grouping of all global tags
func (s *TagGroupSvc) GroupTags(ctx context.Context) error {
	startTime := time.Now()

	// Get all global tags
	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list global tags: %w", err)
	}

	tagCountBefore := len(tags)
	if tagCountBefore == 0 {
		s.logger.Info("no tags to group")
		return nil
	}

	// Extract tag names
	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagNames[i] = tag.Name
	}

	// Use LLM to group tags
	groups, err := s.performGrouping(ctx, tagNames)
	if err != nil {
		return fmt.Errorf("perform grouping: %w", err)
	}

	// Clear existing groups
	if err := s.groupRepo.DeleteAll(ctx); err != nil {
		return fmt.Errorf("delete existing groups: %w", err)
	}

	// Create new groups and update tag group assignments
	for _, group := range groups {
		// Create group
		if err := s.groupRepo.Create(ctx, &group); err != nil {
			s.logger.Warn("failed to create group", zap.String("name", group.Name), zap.Error(err))
			continue
		}

		// Update tags in this group
		for _, tagName := range group.Tags {
			tag, err := s.tagRepo.GetByName(ctx, tagName)
			if err != nil {
				s.logger.Warn("failed to get tag", zap.String("name", tagName), zap.Error(err))
				continue
			}
			if tag == nil {
				continue
			}

			tag.GroupID = group.ID
			// Note: We're not updating the tag directly since we don't have an Update method
			// In a real implementation, we'd need to add an Update method or handle this differently
			// For now, we'll recreate the tag with the new group ID
			if err := s.tagRepo.Create(ctx, tag); err != nil {
				s.logger.Warn("failed to update tag group", zap.String("tag", tagName), zap.Error(err))
			}
		}
	}

	duration := time.Since(startTime).Milliseconds()

	// Log refresh
	if err := s.logRepo.Create(ctx, tagCountBefore, tagCountBefore, len(groups), duration); err != nil {
		s.logger.Warn("failed to create group refresh log", zap.Error(err))
	}

	s.logger.Info("completed tag grouping",
		zap.Int("tags", tagCountBefore),
		zap.Int("groups", len(groups)),
		zap.Int64("duration_ms", duration))

	return nil
}

// ShouldRefresh checks if grouping should be performed
func (s *TagGroupSvc) ShouldRefresh(ctx context.Context) (bool, error) {
	// Get current tag count
	currentCount, err := s.tagRepo.GetCount(ctx)
	if err != nil {
		return false, fmt.Errorf("get tag count: %w", err)
	}

	// Get last refresh log
	lastLog, err := s.logRepo.GetLastRefresh(ctx)
	if err != nil {
		return false, fmt.Errorf("get last refresh: %w", err)
	}

	// No previous refresh, should refresh
	if lastLog == nil {
		return true, nil
	}

	// Check if tag count changed
	if currentCount != lastLog.TagCountAfter {
		return true, nil
	}

	// Check if enough time passed (30 minutes)
	lastRefreshTime, ok := lastLog.CreatedAt.(time.Time)
	if !ok {
		return true, nil
	}

	if time.Since(lastRefreshTime) > 30*time.Minute {
		return true, nil
	}

	return false, nil
}

// GetL1Groups retrieves all Level 1 groups
func (s *TagGroupSvc) GetL1Groups(ctx context.Context) ([]types.TagGroup, error) {
	return s.groupRepo.List(ctx)
}

// GetL2TagsByGroup retrieves Level 2 tags belonging to a group
func (s *TagGroupSvc) GetL2TagsByGroup(ctx context.Context, groupID string) ([]types.Tag, error) {
	return s.tagRepo.ListByGroup(ctx, groupID)
}

// FilterL2TagsByQuery uses LLM to filter L2 tags based on query
func (s *TagGroupSvc) FilterL2TagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	// Build tag list for prompt
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

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		s.logger.Error("LLM tag filter failed", zap.Error(err))
		return s.fallbackTagFilter(tags), nil
	}

	return s.parseTagFilterResults(response), nil
}

// performGrouping uses LLM to group tags into categories
func (s *TagGroupSvc) performGrouping(ctx context.Context, tags []string) ([]types.TagGroup, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	// Build tag list
	tagList := strings.Join(tags, "\n")

	// Determine target number of groups based on tag count
	// Aim for roughly 5-15 tags per group
	targetGroups := len(tags) / 10
	if targetGroups < 3 {
		targetGroups = 3
	}
	if targetGroups > 10 {
		targetGroups = 10
	}

	prompt := fmt.Sprintf(`Group the following tags into %d categories. Each category should have a clear theme and contain related tags.
Aim for balanced distribution (each group should have roughly similar number of tags).

Tags to group:
%s

Return a JSON array where each element has:
- "name": category name (2-4 words)
- "description": brief description (max 100 chars)
- "tags": array of tag names belonging to this category

Response format (JSON only):
[
  {
    "name": "Category Name",
    "description": "Description of this category",
    "tags": ["tag1", "tag2", ...]
  },
  ...
]

Make sure every tag appears in exactly one category.`, targetGroups, tagList)

	response, err := s.provider.Generate(ctx, prompt, 3000)
	if err != nil {
		return nil, fmt.Errorf("LLM grouping failed: %w", err)
	}

	return s.parseGroupResponse(response)
}

// parseGroupResponse parses the LLM grouping response
func (s *TagGroupSvc) parseGroupResponse(response string) ([]types.TagGroup, error) {
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
		return nil, fmt.Errorf("failed to parse group JSON: %w", err)
	}

	groups := make([]types.TagGroup, len(rawGroups))
	for i, rg := range rawGroups {
		groups[i] = types.TagGroup{
			Name:        rg.Name,
			Description: rg.Description,
			Tags:        rg.Tags,
		}
	}

	return groups, nil
}

// parseTagFilterResults parses tag filter results from LLM response
func (s *TagGroupSvc) parseTagFilterResults(response string) []types.TagFilterResult {
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

	// Sort by relevance descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})

	return results
}

// fallbackTagFilter returns all tags with equal relevance
func (s *TagGroupSvc) fallbackTagFilter(tags []types.Tag) []types.TagFilterResult {
	results := make([]types.TagFilterResult, len(tags))
	for i, tag := range tags {
		results[i] = types.TagFilterResult{
			Tag:       tag.Name,
			Relevance: 0.5,
		}
	}
	return results
}

var _ service.ITagGroupService = (*TagGroupSvc)(nil)
