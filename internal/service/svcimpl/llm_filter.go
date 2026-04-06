package svcimpl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// LLMFilterSvc implements service.ILLMFilter
type LLMFilterSvc struct {
	provider client.ILLMProvider
	logger   *zap.Logger
}

// NewLLMFilterSvc creates a new LLM filter service
func NewLLMFilterSvc(provider client.ILLMProvider, logger *zap.Logger) *LLMFilterSvc {
	return &LLMFilterSvc{
		provider: provider,
		logger:   logger,
	}
}

// FilterTopics implements ILLMFilter.FilterTopics
func (s *LLMFilterSvc) FilterTopics(ctx context.Context, query string, topics []types.TopicSummary) ([]types.LLMFilterResult, error) {
	if len(topics) == 0 {
		return nil, nil
	}

	// Build prompt
	var topicList strings.Builder
	for _, t := range topics {
		fmt.Fprintf(&topicList, "[ID: %s] Name: %s | Description: %s\n", t.ID, t.Name, t.Description)
	}

	prompt := fmt.Sprintf(`Given the user query: "%s"

Evaluate the relevance of each topic below. Return a JSON array of objects with fields "id" and "relevance" (0.0-1.0 score).
Only include topics with relevance >= 0.7. Sort by relevance descending.

Topics:
%s

Response format (JSON only, no markdown):
[
  {"id": "topic_id_1", "relevance": 0.95},
  {"id": "topic_id_2", "relevance": 0.82}
]`, query, topicList.String())

	response, err := s.provider.Generate(ctx, prompt, 1000)
	if err != nil {
		s.logger.Error("failed to filter topics", zap.Error(err))
		return nil, err
	}

	return s.parseFilterResults(response), nil
}

// FilterDocuments implements ILLMFilter.FilterDocuments
func (s *LLMFilterSvc) FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Build prompt with truncated content
	var docList strings.Builder
	for _, d := range docs {
		content := truncateContent(d.Content, 500)
		fmt.Fprintf(&docList, "[ID: %s] Title: %s | Content: %s\n\n", d.ID, d.Title, content)
	}

	prompt := fmt.Sprintf(`Given the user query: "%s"

Evaluate the relevance of each document below. Return a JSON array of objects with fields "id" and "relevance" (0.0-1.0 score).
Only include documents with relevance >= 0.6. Sort by relevance descending.

Documents:
%s

Response format (JSON only, no markdown):
[
  {"id": "doc_id_1", "relevance": 0.88},
  {"id": "doc_id_2", "relevance": 0.75}
]`, query, docList.String())

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		s.logger.Error("failed to filter documents", zap.Error(err))
		return nil, err
	}

	return s.parseFilterResults(response), nil
}

// FilterSummaries implements ILLMFilter.FilterSummaries
func (s *LLMFilterSvc) FilterSummaries(ctx context.Context, query string, summaries []types.Summary, tier types.SummaryTier) ([]types.LLMFilterResult, error) {
	if len(summaries) == 0 {
		return nil, nil
	}

	// Build prompt
	var summaryList strings.Builder
	for _, sum := range summaries {
		fmt.Fprintf(&summaryList, "[Path: %s] Content: %s\n\n", sum.Path, truncateContent(sum.Content, 400))
	}

	threshold := 0.5
	if tier == types.TierChapter {
		threshold = 0.55
	}

	prompt := fmt.Sprintf(`Given the user query: "%s"

Evaluate the relevance of each %s summary below. Return a JSON array of objects with fields "id" (the path) and "relevance" (0.0-1.0 score).
Only include items with relevance >= %.2f. Sort by relevance descending.

Summaries:
%s

Response format (JSON only, no markdown):
[
  {"id": "doc_001/chapter_1", "relevance": 0.82},
  {"id": "doc_001/chapter_2/paragraph_3", "relevance": 0.71}
]`, query, tier, threshold, summaryList.String())

	response, err := s.provider.Generate(ctx, prompt, 2000)
	if err != nil {
		s.logger.Error("failed to filter summaries", zap.Error(err))
		return nil, err
	}

	return s.parseFilterResults(response), nil
}

// parseFilterResults parses LLM JSON response to filter results
func (s *LLMFilterSvc) parseFilterResults(response string) []types.LLMFilterResult {
	// Extract JSON array from response
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		s.logger.Warn("invalid filter response format", zap.String("response", response))
		return nil
	}

	jsonStr := response[start : end+1]

	// Simple parsing of JSON array
	var results []types.LLMFilterResult
	
	// Split by "{\"id\":" to get individual objects
	parts := strings.Split(jsonStr, `{"id":`)
	for i, part := range parts {
		if i == 0 {
			continue // Skip the opening bracket part
		}
		
		// Extract id
		idEnd := strings.Index(part, `",`)
		if idEnd == -1 {
			idEnd = strings.Index(part, `"`)
			if idEnd == -1 {
				continue
			}
		}
		id := strings.Trim(strings.TrimSpace(part[:idEnd]), `"`)
		
		// Extract relevance
		relIdx := strings.Index(part, `"relevance":`)
		if relIdx == -1 {
			continue
		}
		relPart := part[relIdx+len(`"relevance":`):]
		relEnd := strings.IndexAny(relPart, ",}]")
		if relEnd == -1 {
			relEnd = len(relPart)
		}
		relStr := strings.TrimSpace(relPart[:relEnd])
		relevance, err := strconv.ParseFloat(relStr, 64)
		if err != nil {
			s.logger.Warn("failed to parse relevance", zap.String("value", relStr))
			continue
		}
		
		results = append(results, types.LLMFilterResult{
			ID:        id,
			Relevance: relevance,
		})
	}

	return results
}

var _ service.ILLMFilter = (*LLMFilterSvc)(nil)
