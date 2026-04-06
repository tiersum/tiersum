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

// TagClusteringSvc implements service.ITagClusteringService
type TagClusteringSvc struct {
	globalTagRepo storage.IGlobalTagRepository
	clusterRepo   storage.ITagClusterRepository
	logRepo       storage.IClusterRefreshLogRepository
	provider      client.ILLMProvider
	logger        *zap.Logger
}

// NewTagClusteringSvc creates a new tag clustering service
func NewTagClusteringSvc(
	globalTagRepo storage.IGlobalTagRepository,
	clusterRepo storage.ITagClusterRepository,
	logRepo storage.IClusterRefreshLogRepository,
	provider client.ILLMProvider,
	logger *zap.Logger,
) *TagClusteringSvc {
	return &TagClusteringSvc{
		globalTagRepo: globalTagRepo,
		clusterRepo:   clusterRepo,
		logRepo:       logRepo,
		provider:      provider,
		logger:        logger,
	}
}

// ClusterTags performs LLM-based clustering of all global tags
func (s *TagClusteringSvc) ClusterTags(ctx context.Context) error {
	startTime := time.Now()

	// Get all global tags
	tags, err := s.globalTagRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list global tags: %w", err)
	}

	tagCountBefore := len(tags)
	if tagCountBefore == 0 {
		s.logger.Info("no tags to cluster")
		return nil
	}

	// Extract tag names
	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagNames[i] = tag.Name
	}

	// Use LLM to cluster tags
	clusters, err := s.performClustering(ctx, tagNames)
	if err != nil {
		return fmt.Errorf("perform clustering: %w", err)
	}

	// Clear existing clusters
	if err := s.clusterRepo.DeleteAll(ctx); err != nil {
		return fmt.Errorf("delete existing clusters: %w", err)
	}

	// Create new clusters and update tag cluster assignments
	for _, cluster := range clusters {
		// Create cluster
		if err := s.clusterRepo.Create(ctx, &cluster); err != nil {
			s.logger.Warn("failed to create cluster", zap.String("name", cluster.Name), zap.Error(err))
			continue
		}

		// Update tags in this cluster
		for _, tagName := range cluster.Tags {
			tag, err := s.globalTagRepo.GetByName(ctx, tagName)
			if err != nil {
				s.logger.Warn("failed to get tag", zap.String("name", tagName), zap.Error(err))
				continue
			}
			if tag == nil {
				continue
			}

			tag.ClusterID = cluster.ID
			// Note: We're not updating the tag directly since we don't have an Update method
			// In a real implementation, we'd need to add an Update method or handle this differently
			// For now, we'll recreate the tag with the new cluster ID
			if err := s.globalTagRepo.Create(ctx, tag); err != nil {
				s.logger.Warn("failed to update tag cluster", zap.String("tag", tagName), zap.Error(err))
			}
		}
	}

	duration := time.Since(startTime).Milliseconds()

	// Log refresh
	if err := s.logRepo.Create(ctx, tagCountBefore, tagCountBefore, len(clusters), duration); err != nil {
		s.logger.Warn("failed to create cluster refresh log", zap.Error(err))
	}

	s.logger.Info("completed tag clustering",
		zap.Int("tags", tagCountBefore),
		zap.Int("clusters", len(clusters)),
		zap.Int64("duration_ms", duration))

	return nil
}

// ShouldRefresh checks if clustering should be performed
func (s *TagClusteringSvc) ShouldRefresh(ctx context.Context) (bool, error) {
	// Get current tag count
	currentCount, err := s.globalTagRepo.GetCount(ctx)
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

// GetL1Clusters retrieves all Level 1 clusters
func (s *TagClusteringSvc) GetL1Clusters(ctx context.Context) ([]types.TagCluster, error) {
	return s.clusterRepo.List(ctx)
}

// GetL2TagsByCluster retrieves Level 2 tags belonging to a cluster
func (s *TagClusteringSvc) GetL2TagsByCluster(ctx context.Context, clusterID string) ([]types.GlobalTag, error) {
	return s.globalTagRepo.ListByCluster(ctx, clusterID)
}

// FilterL2TagsByQuery uses LLM to filter L2 tags based on query
func (s *TagClusteringSvc) FilterL2TagsByQuery(ctx context.Context, query string, tags []types.GlobalTag) ([]types.TagFilterResult, error) {
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

// performClustering uses LLM to cluster tags into categories
func (s *TagClusteringSvc) performClustering(ctx context.Context, tags []string) ([]types.TagCluster, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	// Build tag list
	tagList := strings.Join(tags, "\n")

	// Determine target number of clusters based on tag count
	// Aim for roughly 5-15 tags per cluster
	targetClusters := len(tags) / 10
	if targetClusters < 3 {
		targetClusters = 3
	}
	if targetClusters > 10 {
		targetClusters = 10
	}

	prompt := fmt.Sprintf(`Cluster the following tags into %d categories. Each category should have a clear theme and contain related tags.
Aim for balanced distribution (each cluster should have roughly similar number of tags).

Tags to cluster:
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

Make sure every tag appears in exactly one category.`, targetClusters, tagList)

	response, err := s.provider.Generate(ctx, prompt, 3000)
	if err != nil {
		return nil, fmt.Errorf("LLM clustering failed: %w", err)
	}

	return s.parseClusterResponse(response)
}

// parseClusterResponse parses the LLM clustering response
func (s *TagClusteringSvc) parseClusterResponse(response string) ([]types.TagCluster, error) {
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var rawClusters []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawClusters); err != nil {
		return nil, fmt.Errorf("failed to parse cluster JSON: %w", err)
	}

	clusters := make([]types.TagCluster, len(rawClusters))
	for i, rc := range rawClusters {
		clusters[i] = types.TagCluster{
			Name:        rc.Name,
			Description: rc.Description,
			Tags:        rc.Tags,
		}
	}

	return clusters, nil
}

// parseTagFilterResults parses tag filter results from LLM response
func (s *TagClusteringSvc) parseTagFilterResults(response string) []types.TagFilterResult {
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
func (s *TagClusteringSvc) fallbackTagFilter(tags []types.GlobalTag) []types.TagFilterResult {
	results := make([]types.TagFilterResult, len(tags))
	for i, tag := range tags {
		results[i] = types.TagFilterResult{
			Tag:       tag.Name,
			Relevance: 0.5,
		}
	}
	return results
}

var _ service.ITagClusteringService = (*TagClusteringSvc)(nil)
