package svcimpl

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// QuerySvc implements service.IQueryService
type QuerySvc struct {
	docRepo       storage.IDocumentRepository
	summaryRepo   storage.ISummaryRepository
	globalTagRepo storage.IGlobalTagRepository
	clusterRepo   storage.ITagClusterRepository
	summarizer    service.ISummarizer
	logger        *zap.Logger
}

// NewQuerySvc creates a new query service
func NewQuerySvc(
	docRepo storage.IDocumentRepository,
	summaryRepo storage.ISummaryRepository,
	globalTagRepo storage.IGlobalTagRepository,
	clusterRepo storage.ITagClusterRepository,
	summarizer service.ISummarizer,
	logger *zap.Logger,
) *QuerySvc {
	return &QuerySvc{
		docRepo:       docRepo,
		summaryRepo:   summaryRepo,
		globalTagRepo: globalTagRepo,
		clusterRepo:   clusterRepo,
		summarizer:    summarizer,
		logger:        logger,
	}
}

// Query performs hierarchical query (legacy interface)
func (s *QuerySvc) Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error) {
	req := types.ProgressiveQueryRequest{
		Question:   question,
		MaxResults: 100,
	}

	resp, err := s.ProgressiveQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to legacy format
	var results []types.QueryResult
	for _, item := range resp.Results {
		if depth == "" || item.Tier == depth {
			results = append(results, types.QueryResult{
				DocumentID:    item.ID,
				DocumentTitle: item.Title,
				Tier:          item.Tier,
				Path:          item.Path,
				Content:       item.Content,
				Relevance:     item.Relevance,
			})
		}
	}

	return results, nil
}

// ProgressiveQuery implements the new two-level tag-based progressive query
// 5.1 Query L1 clusters + keyword -> LLM filter -> L2 tags
// 5.2 L2 tags -> query top 100 doc summaries -> LLM filter -> docs
// 5.3 Docs -> query chapter summaries -> LLM filter -> chapters
// 5.4 Chapters -> query source content
func (s *QuerySvc) ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	response := &types.ProgressiveQueryResponse{
		Question: req.Question,
		Steps:    []types.ProgressiveQueryStep{},
	}

	// Step 5.1: Get L1 clusters and filter to get relevant L2 tags
	step1Start := time.Now()
	l2Tags, err := s.filterL2Tags(ctx, req.Question)
	if err != nil {
		s.logger.Error("failed to filter L2 tags", zap.Error(err))
		return nil, fmt.Errorf("filter L2 tags: %w", err)
	}
	response.Steps = append(response.Steps, types.ProgressiveQueryStep{
		Step:     "L2_tags",
		Input:    req.Question,
		Output:   l2Tags,
		Duration: time.Since(step1Start).Milliseconds(),
	})

	// Step 5.2: Query documents by L2 tags and filter
	step2Start := time.Now()
	docs, err := s.queryAndFilterDocuments(ctx, req.Question, l2Tags, req.MaxResults)
	if err != nil {
		s.logger.Error("failed to query documents", zap.Error(err))
		return nil, fmt.Errorf("query documents: %w", err)
	}
	response.Steps = append(response.Steps, types.ProgressiveQueryStep{
		Step:     "documents",
		Input:    l2Tags,
		Output:   len(docs),
		Duration: time.Since(step2Start).Milliseconds(),
	})

	// Step 5.3: Query chapters by docs and filter
	step3Start := time.Now()
	chapters, err := s.queryAndFilterChapters(ctx, req.Question, docs)
	if err != nil {
		s.logger.Error("failed to query chapters", zap.Error(err))
		return nil, fmt.Errorf("query chapters: %w", err)
	}
	response.Steps = append(response.Steps, types.ProgressiveQueryStep{
		Step:     "chapters",
		Input:    len(docs),
		Output:   len(chapters),
		Duration: time.Since(step3Start).Milliseconds(),
	})

	// Step 5.4: Build final results with source content
	results := s.buildResults(chapters)
	response.Results = results

	s.logger.Info("progressive query completed",
		zap.String("question", req.Question),
		zap.Int("l2_tags", len(l2Tags)),
		zap.Int("docs", len(docs)),
		zap.Int("chapters", len(chapters)),
		zap.Int("results", len(results)))

	return response, nil
}

// filterL2Tags gets all L2 tags and filters them by query using LLM
func (s *QuerySvc) filterL2Tags(ctx context.Context, query string) ([]string, error) {
	// Get all global tags
	allTags, err := s.globalTagRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list global tags: %w", err)
	}

	if len(allTags) == 0 {
		return nil, nil
	}

	// Try to filter using LLM
	filterResults, err := s.summarizer.(interface {
		FilterL2TagsByQuery(ctx context.Context, query string, tags []types.GlobalTag) ([]types.TagFilterResult, error)
	}).FilterL2TagsByQuery(ctx, query, allTags)

	if err != nil {
		s.logger.Warn("LLM tag filter failed, using all tags", zap.Error(err))
		// Fallback: return all tag names
		tagNames := make([]string, len(allTags))
		for i, tag := range allTags {
			tagNames[i] = tag.Name
		}
		return tagNames, nil
	}

	// Extract tag names from filter results
	tagNames := make([]string, 0, len(filterResults))
	for _, result := range filterResults {
		if result.Relevance >= 0.5 {
			tagNames = append(tagNames, result.Tag)
		}
	}

	return tagNames, nil
}

// queryAndFilterDocuments queries documents by tags and filters by query
func (s *QuerySvc) queryAndFilterDocuments(ctx context.Context, query string, tags []string, limit int) ([]types.Document, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	// Query documents by tags (OR logic - documents matching ANY tag)
	docs, err := s.docRepo.ListByTags(ctx, tags, limit)
	if err != nil {
		return nil, fmt.Errorf("list documents by tags: %w", err)
	}

	if len(docs) == 0 {
		return nil, nil
	}

	// Filter documents by query using LLM
	filterResults, err := s.summarizer.FilterDocuments(ctx, query, docs)
	if err != nil {
		s.logger.Warn("LLM document filter failed, returning all documents", zap.Error(err))
		return docs, nil
	}

	// Build doc map for lookup
	docMap := make(map[string]types.Document)
	for _, doc := range docs {
		docMap[doc.ID] = doc
	}

	// Sort by relevance and filter
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	var filteredDocs []types.Document
	for _, fr := range filterResults {
		if fr.Relevance >= 0.5 {
			if doc, ok := docMap[fr.ID]; ok {
				filteredDocs = append(filteredDocs, doc)
			}
		}
	}

	return filteredDocs, nil
}

// queryAndFilterChapters queries chapters by documents and filters by query
func (s *QuerySvc) queryAndFilterChapters(ctx context.Context, query string, docs []types.Document) ([]types.Summary, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Get all chapters for these documents
	var allChapters []types.Summary
	for _, doc := range docs {
		chapters, err := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, doc.ID)
		if err != nil {
			s.logger.Warn("failed to get document chapters", zap.String("doc_id", doc.ID), zap.Error(err))
			continue
		}
		allChapters = append(allChapters, chapters...)
	}

	if len(allChapters) == 0 {
		return nil, nil
	}

	// Filter chapters by query using LLM
	filterResults, err := s.summarizer.FilterChapters(ctx, query, allChapters)
	if err != nil {
		s.logger.Warn("LLM chapter filter failed, returning all chapters", zap.Error(err))
		return allChapters, nil
	}

	// Build chapter map
	chapterMap := make(map[string]types.Summary)
	for _, ch := range allChapters {
		chapterMap[ch.Path] = ch
	}

	// Sort by relevance and filter
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	var filteredChapters []types.Summary
	for _, fr := range filterResults {
		if fr.Relevance >= 0.5 {
			if ch, ok := chapterMap[fr.ID]; ok {
				filteredChapters = append(filteredChapters, ch)
			}
		}
	}

	return filteredChapters, nil
}

// buildResults builds final query results from chapters
func (s *QuerySvc) buildResults(chapters []types.Summary) []types.QueryItem {
	results := make([]types.QueryItem, len(chapters))
	for i, ch := range chapters {
		results[i] = types.QueryItem{
			ID:        ch.DocumentID,
			Title:     extractTitleFromPath(ch.Path),
			Content:   ch.Content,
			Tier:      types.TierChapter,
			Path:      ch.Path,
			Relevance: 1.0, // Already filtered by LLM
			IsSource:  false,
		}
	}
	return results
}

// extractTitleFromPath extracts a readable title from path
func extractTitleFromPath(path string) string {
	parts := splitPath(path)
	if len(parts) == 0 {
		return path
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[len(parts)-1]
}

// splitPath splits path by "/"
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	for _, p := range splitByChar(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// splitByChar splits string by character
func splitByChar(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

var _ service.IQueryService = (*QuerySvc)(nil)
