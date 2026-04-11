package svcimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/metrics"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// L2TagThreshold is the threshold for adaptive L1/L2 filtering
// If L2 tag count < threshold: directly filter all L2 tags with LLM (skip L1)
// If L2 tag count >= threshold: use L1 -> L2 two-level filtering
const L2TagThreshold = 200

// QuerySvc implements service.IQueryService
type QuerySvc struct {
	docRepo     storage.IDocumentRepository
	summaryRepo storage.ISummaryRepository
	tagRepo     storage.ITagRepository
	groupRepo   storage.ITagGroupRepository
	summarizer  service.ISummarizer
	coldIndex   storage.IColdIndex
	llm         client.ILLMProvider
	logger      *zap.Logger
}

// NewQuerySvc creates a new query service
func NewQuerySvc(
	docRepo storage.IDocumentRepository,
	summaryRepo storage.ISummaryRepository,
	tagRepo storage.ITagRepository,
	groupRepo storage.ITagGroupRepository,
	summarizer service.ISummarizer,
	coldIndex storage.IColdIndex,
	llm client.ILLMProvider,
	logger *zap.Logger,
) *QuerySvc {
	return &QuerySvc{
		docRepo:     docRepo,
		summaryRepo: summaryRepo,
		tagRepo:     tagRepo,
		groupRepo:   groupRepo,
		summarizer:  summarizer,
		coldIndex:   coldIndex,
		llm:         llm,
		logger:      logger,
	}
}

// ProgressiveQuery implements the two-level tag-based progressive query
// 1 Query L1 tag groups + keyword -> LLM filter -> L2 tags
// 2 L2 tags -> query top 100 doc summaries -> LLM filter -> docs
// 3 Docs -> query chapter summaries -> LLM filter -> chapters
// 4 Chapters -> query source content
// 5 Cold docs -> BM25 + vector search (parallel with hot path)
func (s *QuerySvc) ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	response := &types.ProgressiveQueryResponse{
		Question: req.Question,
		Steps:    []types.ProgressiveQueryStep{},
	}

	// Run hot path and cold path concurrently
	type hotResult struct {
		results []types.QueryItem
		steps   []types.ProgressiveQueryStep
		err     error
	}
	type coldResult struct {
		results []types.QueryItem
		step    types.ProgressiveQueryStep
		err     error
	}

	hotChan := make(chan hotResult, 1)
	coldChan := make(chan coldResult, 1)

	// Hot path: Tag-based progressive query
	go func() {
		results, steps, err := s.queryHotPath(ctx, req)
		hotChan <- hotResult{results: results, steps: steps, err: err}
	}()

	// Cold path: BM25 + vector search
	go func() {
		results, step, err := s.queryColdPath(ctx, req)
		coldChan <- coldResult{results: results, step: step, err: err}
	}()

	// Collect results
	hotRes := <-hotChan
	coldRes := <-coldChan

	// Process hot path results
	if hotRes.err != nil {
		s.logger.Error("hot path query failed", zap.Error(hotRes.err))
	} else {
		response.Steps = append(response.Steps, hotRes.steps...)
	}

	// Process cold path results
	if coldRes.err != nil {
		s.logger.Error("cold path query failed", zap.Error(coldRes.err))
	} else {
		response.Steps = append(response.Steps, coldRes.step)
	}

	// Merge results from both paths
	mergedResults := s.mergeHotAndColdResults(hotRes.results, coldRes.results, req.MaxResults)
	response.Results = mergedResults
	response.Answer = s.generateProgressiveAnswer(ctx, req.Question, mergedResults)

	s.logger.Info("progressive query completed",
		zap.String("question", req.Question),
		zap.Int("hot_results", len(hotRes.results)),
		zap.Int("cold_results", len(coldRes.results)),
		zap.Int("total_results", len(mergedResults)),
		zap.Bool("has_answer", response.Answer != ""))

	return response, nil
}

// queryHotPath performs the hot document query path (tag-based progressive)
func (s *QuerySvc) queryHotPath(ctx context.Context, req types.ProgressiveQueryRequest) ([]types.QueryItem, []types.ProgressiveQueryStep, error) {
	start := time.Now()
	var steps []types.ProgressiveQueryStep

	// Step 1: Get L1 tag groups and filter to get relevant L2 tags
	step1Start := time.Now()
	l2Tags, err := s.filterL2Tags(ctx, req.Question)
	if err != nil {
		s.logger.Error("failed to filter L2 tags", zap.Error(err))
		return nil, nil, fmt.Errorf("filter L2 tags: %w", err)
	}
	steps = append(steps, types.ProgressiveQueryStep{
		Step:     "L2_tags",
		Input:    req.Question,
		Output:   l2Tags,
		Duration: time.Since(step1Start).Milliseconds(),
	})

	// Step 2: Query documents by L2 tags and filter
	step2Start := time.Now()
	docs, err := s.queryAndFilterDocuments(ctx, req.Question, l2Tags, req.MaxResults)
	if err != nil {
		s.logger.Error("failed to query documents", zap.Error(err))
		return nil, nil, fmt.Errorf("query documents: %w", err)
	}
	steps = append(steps, types.ProgressiveQueryStep{
		Step:     "documents",
		Input:    l2Tags,
		Output:   len(docs),
		Duration: time.Since(step2Start).Milliseconds(),
	})

	// Track document access for hot/cold management
	s.trackDocumentAccess(ctx, docs)

	// Step 3: Query chapters by docs and filter
	step3Start := time.Now()
	chapters, err := s.queryAndFilterChapters(ctx, req.Question, docs)
	if err != nil {
		s.logger.Error("failed to query chapters", zap.Error(err))
		return nil, nil, fmt.Errorf("query chapters: %w", err)
	}
	steps = append(steps, types.ProgressiveQueryStep{
		Step:     "chapters",
		Input:    len(docs),
		Output:   len(chapters),
		Duration: time.Since(step3Start).Milliseconds(),
	})

	statusByID := make(map[string]types.DocumentStatus, len(docs))
	for _, d := range docs {
		statusByID[d.ID] = d.Status
	}
	results := s.buildResults(chapters, statusByID)

	// Record metrics
	metrics.RecordQueryLatency(metrics.QueryPathHot, time.Since(start).Seconds(), len(results))

	return results, steps, nil
}

// queryColdPath performs the cold document query path (cold index search).
func (s *QuerySvc) queryColdPath(ctx context.Context, req types.ProgressiveQueryRequest) ([]types.QueryItem, types.ProgressiveQueryStep, error) {
	start := time.Now()

	if s.coldIndex == nil {
		return nil, types.ProgressiveQueryStep{
			Step:     "cold_docs",
			Input:    req.Question,
			Output:   0,
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}

	searchResults, err := s.coldIndex.Search(ctx, req.Question, req.MaxResults/2)
	if err != nil {
		return nil, types.ProgressiveQueryStep{}, fmt.Errorf("cold index search failed: %w", err)
	}

	// Convert search results to query items
	var results []types.QueryItem
	for _, sr := range searchResults {
		path := sr.Path
		if path == "" {
			path = sr.DocumentID + "/full"
		}
		title := extractTitleFromPath(path)
		if title == "" || title == sr.DocumentID {
			title = sr.Title
		}
		results = append(results, types.QueryItem{
			ID:            sr.DocumentID,
			Title:         title,
			Content:       sr.Content,
			Tier:          types.TierChapter,
			Path:          path,
			Relevance:     sr.Score,
			IsSource:      false,
			Status:        types.DocStatusCold,
			ContentSource: sr.Source,
		})
	}

	step := types.ProgressiveQueryStep{
		Step:     "cold_docs",
		Input:    req.Question,
		Output:   len(results),
		Duration: time.Since(start).Milliseconds(),
	}

	return results, step, nil
}

// mergeHotAndColdResults merges results from hot and cold paths
func (s *QuerySvc) mergeHotAndColdResults(hotResults, coldResults []types.QueryItem, maxResults int) []types.QueryItem {
	// Use a map to deduplicate by document ID
	resultMap := make(map[string]*types.QueryItem)

	// Add hot results first (they have higher quality from LLM filtering)
	for i := range hotResults {
		r := &hotResults[i]
		resultMap[r.ID] = r
	}

	// Add cold results, avoiding duplicates
	for i := range coldResults {
		r := &coldResults[i]
		if existing, ok := resultMap[r.ID]; ok {
			// Document exists in both - boost relevance if cold result is also found
			if r.Relevance > existing.Relevance {
				existing.Relevance = r.Relevance
			}
		} else {
			resultMap[r.ID] = r
		}
	}

	// Convert map to slice
	results := make([]types.QueryItem, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// Sort by relevance descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})

	// Return top results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// trackDocumentAccess increments query count for accessed documents
// and triggers promotion for cold documents that exceed the threshold
func (s *QuerySvc) trackDocumentAccess(ctx context.Context, docs []types.Document) {
	for _, doc := range docs {
		// Increment query count in background
		go func(docID string, status types.DocumentStatus, queryCount int) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := s.docRepo.IncrementQueryCount(bgCtx, docID); err != nil {
				s.logger.Warn("failed to increment query count",
					zap.String("doc_id", docID),
					zap.Error(err))
				return
			}

			threshold := config.ColdPromotionThreshold()
			if status == types.DocStatusCold && queryCount+1 >= threshold {
				select {
				case job.PromoteQueue <- docID:
					s.logger.Info("queued cold document for promotion",
						zap.String("doc_id", docID),
						zap.Int("query_count", queryCount+1))
				default:
					s.logger.Warn("promotion queue full, document not queued",
						zap.String("doc_id", docID))
				}
			}
		}(doc.ID, doc.Status, doc.QueryCount)
	}
}

// filterL2Tags gets all L2 tags and filters them by query using LLM
// Implements adaptive two-level filtering:
// - If L2 tag count < L2TagThreshold: directly filter all L2 tags with LLM (skip L1)
// - If L2 tag count >= L2TagThreshold: use L1 -> L2 two-level filtering
func (s *QuerySvc) filterL2Tags(ctx context.Context, query string) ([]string, error) {
	// Get all global tags (L2 tags)
	allTags, err := s.tagRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list global tags: %w", err)
	}

	if len(allTags) == 0 {
		return nil, nil
	}

	// Adaptive filtering based on tag count
	if len(allTags) < L2TagThreshold {
		// Direct L2 filtering: skip L1, filter all L2 tags directly
		s.logger.Info("adaptive filtering: direct L2 filter (tag count below threshold)",
			zap.Int("tag_count", len(allTags)),
			zap.Int("threshold", L2TagThreshold))
		return s.filterL2TagsDirect(ctx, query, allTags)
	}

	// Two-level filtering: L1 -> L2
	s.logger.Info("adaptive filtering: L1 -> L2 two-level filter (tag count above threshold)",
		zap.Int("tag_count", len(allTags)),
		zap.Int("threshold", L2TagThreshold))
	return s.filterL2TagsTwoLevel(ctx, query)
}

// filterL2TagsDirect directly filters all L2 tags using LLM (skip L1)
func (s *QuerySvc) filterL2TagsDirect(ctx context.Context, query string, tags []types.Tag) ([]string, error) {
	// Try to filter using LLM via type assertion
	type filterer interface {
		FilterL2TagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error)
	}

	f, ok := s.summarizer.(filterer)
	if !ok {
		s.logger.Warn("summarizer does not support FilterL2TagsByQuery, returning all tags")
		return s.extractTagNames(tags), nil
	}

	filterResults, err := f.FilterL2TagsByQuery(ctx, query, tags)
	if err != nil {
		s.logger.Warn("LLM tag filter failed, using all tags", zap.Error(err))
		return s.extractTagNames(tags), nil
	}

	// Extract tag names from filter results
	return s.extractRelevantTags(filterResults), nil
}

// filterL2TagsTwoLevel performs L1 -> L2 two-level filtering
// 1. Select 1-3 relevant L1 groups using LLM
// 2. Collect all L2 tags from selected groups
// 3. Filter those L2 tags with LLM
func (s *QuerySvc) filterL2TagsTwoLevel(ctx context.Context, query string) ([]string, error) {
	// Step 1: Get all L1 groups and filter to select 1-3 most relevant
	selectedGroups, err := s.filterL1Groups(ctx, query)
	if err != nil {
		s.logger.Warn("L1 group filter failed, falling back to direct L2 filter", zap.Error(err))
		// Fallback: get all tags and filter directly
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterL2TagsDirect(ctx, query, allTags)
	}

	if len(selectedGroups) == 0 {
		s.logger.Warn("no L1 groups selected, falling back to direct L2 filter")
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterL2TagsDirect(ctx, query, allTags)
	}

	s.logger.Info("L1 groups selected", zap.Int("count", len(selectedGroups)))

	// Step 2: Get all L2 tags from selected groups
	groupIDs := make([]string, len(selectedGroups))
	for i, g := range selectedGroups {
		groupIDs[i] = g.ID
	}

	l2Tags, err := s.getL2TagsFromGroups(ctx, groupIDs)
	if err != nil {
		s.logger.Warn("failed to get L2 tags from groups, falling back to direct L2 filter", zap.Error(err))
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterL2TagsDirect(ctx, query, allTags)
	}

	if len(l2Tags) == 0 {
		s.logger.Warn("no L2 tags found in selected groups, falling back to direct L2 filter")
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterL2TagsDirect(ctx, query, allTags)
	}

	s.logger.Info("L2 tags collected from selected groups", zap.Int("count", len(l2Tags)))

	// Step 3: Filter the collected L2 tags with LLM
	return s.filterL2TagsDirect(ctx, query, l2Tags)
}

// filterL1Groups uses LLM to select 1-3 most relevant tag groups (L1) for the query
func (s *QuerySvc) filterL1Groups(ctx context.Context, query string) ([]types.TagGroup, error) {
	// Get all L1 groups
	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tag groups: %w", err)
	}

	if len(groups) == 0 {
		return nil, nil
	}

	// Try to use LLM to filter groups via type assertion
	type groupFilterer interface {
		FilterL1GroupsByQuery(ctx context.Context, query string, groups []types.TagGroup) ([]types.LLMFilterResult, error)
	}

	f, ok := s.summarizer.(groupFilterer)
	if !ok {
		s.logger.Warn("summarizer does not support FilterL1GroupsByQuery, returning all groups")
		return groups, nil
	}

	filterResults, err := f.FilterL1GroupsByQuery(ctx, query, groups)
	if err != nil {
		s.logger.Warn("LLM group filter failed, returning all groups", zap.Error(err))
		return groups, nil
	}

	// Build group map for lookup
	groupMap := make(map[string]types.TagGroup)
	for _, g := range groups {
		groupMap[g.ID] = g
	}

	// Sort by relevance descending
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	// Select top 1-3 groups with relevance >= 0.5
	var selectedGroups []types.TagGroup
	for _, fr := range filterResults {
		if fr.Relevance >= 0.5 && len(selectedGroups) < 3 {
			if g, ok := groupMap[fr.ID]; ok {
				selectedGroups = append(selectedGroups, g)
			}
		}
	}

	return selectedGroups, nil
}

// getL2TagsFromGroups retrieves all L2 tags belonging to the given group IDs
func (s *QuerySvc) getL2TagsFromGroups(ctx context.Context, groupIDs []string) ([]types.Tag, error) {
	var allTags []types.Tag
	seenTags := make(map[string]bool)

	for _, groupID := range groupIDs {
		tags, err := s.tagRepo.ListByGroup(ctx, groupID)
		if err != nil {
			s.logger.Warn("failed to get tags by group", zap.String("group_id", groupID), zap.Error(err))
			continue
		}

		for _, tag := range tags {
			if !seenTags[tag.ID] {
				seenTags[tag.ID] = true
				allTags = append(allTags, tag)
			}
		}
	}

	return allTags, nil
}

// extractTagNames extracts tag names from tags
func (s *QuerySvc) extractTagNames(tags []types.Tag) []string {
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = tag.Name
	}
	return names
}

// extractRelevantTags extracts tag names with relevance >= 0.5 from filter results
func (s *QuerySvc) extractRelevantTags(results []types.TagFilterResult) []string {
	var names []string
	for _, r := range results {
		if r.Relevance >= 0.5 {
			names = append(names, r.Tag)
		}
	}
	return names
}

// queryAndFilterDocuments queries documents by tags and filters by query
// For Hot docs: uses LLM filtering
// For Cold docs: uses simple keyword matching
func (s *QuerySvc) queryAndFilterDocuments(ctx context.Context, query string, tags []string, limit int) ([]types.Document, error) {
	var docs []types.Document
	var err error

	if len(tags) == 0 {
		// No L2 tag names after filtering (empty tag table, or LLM returned no tags above threshold).
		// Scope documents by listing up to limit; hot/cold paths still apply LLM or keyword filtering.
		s.logger.Debug("progressive query: no tag filter results; listing documents as fallback",
			zap.Int("limit", limit))
		docs, err = s.docRepo.ListAll(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("list all documents: %w", err)
		}
	} else {
		// Query documents by tags (OR logic - documents matching ANY tag)
		docs, err = s.docRepo.ListByTags(ctx, tags, limit)
		if err != nil {
			return nil, fmt.Errorf("list documents by tags: %w", err)
		}
	}

	if len(docs) == 0 {
		return nil, nil
	}

	// Separate hot and cold documents
	var hotDocs, coldDocs []types.Document
	for _, doc := range docs {
		if doc.Status == types.DocStatusHot {
			hotDocs = append(hotDocs, doc)
		} else {
			coldDocs = append(coldDocs, doc)
		}
	}

	var filteredDocs []types.Document

	// For hot documents: use LLM filtering
	if len(hotDocs) > 0 {
		hotFiltered, err := s.filterHotDocuments(ctx, query, hotDocs)
		if err != nil {
			s.logger.Warn("LLM document filter failed for hot docs", zap.Error(err))
			filteredDocs = append(filteredDocs, hotDocs...)
		} else {
			filteredDocs = append(filteredDocs, hotFiltered...)
		}
	}

	// For cold documents: use simple keyword matching
	if len(coldDocs) > 0 {
		coldFiltered := s.filterColdDocuments(query, coldDocs)
		filteredDocs = append(filteredDocs, coldFiltered...)
	}

	return filteredDocs, nil
}

// filterHotDocuments filters hot documents using LLM
func (s *QuerySvc) filterHotDocuments(ctx context.Context, query string, docs []types.Document) ([]types.Document, error) {
	filterResults, err := s.summarizer.FilterDocuments(ctx, query, docs)
	if err != nil {
		return nil, err
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

// filterColdDocuments filters cold documents using simple keyword matching
func (s *QuerySvc) filterColdDocuments(query string, docs []types.Document) []types.Document {
	keywords := types.ExtractKeywords(query, 10)
	var filteredDocs []types.Document

	for _, doc := range docs {
		if s.matchesColdDocument(doc, keywords) {
			filteredDocs = append(filteredDocs, doc)
		}
	}

	return filteredDocs
}

// matchesColdDocument checks if a cold document matches the query keywords
func (s *QuerySvc) matchesColdDocument(doc types.Document, keywords []string) bool {
	contentLower := strings.ToLower(doc.Content)
	titleLower := strings.ToLower(doc.Title)

	// Match if any keyword is found in title or content
	for _, keyword := range keywords {
		keywordLower := strings.ToLower(keyword)
		if strings.Contains(titleLower, keywordLower) || strings.Contains(contentLower, keywordLower) {
			return true
		}
	}

	// Also check document tags
	for _, tag := range doc.Tags {
		tagLower := strings.ToLower(tag)
		for _, keyword := range keywords {
			if strings.Contains(tagLower, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}

// queryAndFilterChapters queries chapters by documents and filters by query
func (s *QuerySvc) queryAndFilterChapters(ctx context.Context, query string, docs []types.Document) ([]types.Summary, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Get all chapters for these documents
	var allChapters []types.Summary
	for _, doc := range docs {
		// For cold documents, use the full body as one pseudo-chapter for LLM filtering
		if doc.Status != types.DocStatusHot {
			chapter := s.createColdDocumentChapter(doc, query)
			if chapter != nil {
				allChapters = append(allChapters, *chapter)
			}
			continue
		}

		// For hot documents, get chapters from summary repository
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

// createColdDocumentChapter returns the full cold document body as one chapter (no keyword snippet).
// Progressive cold hits come from the cold index with per-chapter paths; this path is for hot-path chapter filtering only.
func (s *QuerySvc) createColdDocumentChapter(doc types.Document, query string) *types.Summary {
	_ = query
	return &types.Summary{
		ID:         doc.ID + "_cold",
		DocumentID: doc.ID,
		Tier:       types.TierChapter,
		Path:       doc.ID + "/full",
		Content:    doc.Content,
		IsSource:   false,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}
}

// buildResults builds final query results from chapters
func (s *QuerySvc) buildResults(chapters []types.Summary, docStatusByID map[string]types.DocumentStatus) []types.QueryItem {
	results := make([]types.QueryItem, len(chapters))
	for i, ch := range chapters {
		st := docStatusByID[ch.DocumentID]
		results[i] = types.QueryItem{
			ID:            ch.DocumentID,
			Title:         extractTitleFromPath(ch.Path),
			Content:       ch.Content,
			Tier:          types.TierChapter,
			Path:          ch.Path,
			Relevance:     1.0, // Already filtered by LLM
			IsSource:      false,
			Status:        st,
			ContentSource: "chapter_summary",
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
