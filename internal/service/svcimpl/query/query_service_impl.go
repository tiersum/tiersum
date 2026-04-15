package query

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tiersum/tiersum/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl/common"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// CatalogTagThreshold controls adaptive topic→tag filtering for progressive query.
// Below threshold: LLM filters all catalog tags directly (skip topics).
// At or above: LLM picks topics first, then filters tags within those topics.
const CatalogTagThreshold = 200

// NewQueryService constructs the query service implementation.
func NewQueryService(
	docRepo storage.IDocumentRepository,
	chapterRepo storage.IChapterRepository,
	tagRepo storage.ITagRepository,
	topicRepo storage.ITopicRepository,
	filter service.IRelevanceFilter,
	coldIndex storage.IColdIndex,
	llm client.ILLMProvider,
	logger *zap.Logger,
) service.IQueryService {
	return &queryService{
		docRepo:     docRepo,
		chapterRepo: chapterRepo,
		tagRepo:     tagRepo,
		topicRepo:   topicRepo,
		filter:      filter,
		coldIndex:   coldIndex,
		llm:         llm,
		logger:      logger,
	}
}

type queryService struct {
	docRepo     storage.IDocumentRepository
	chapterRepo storage.IChapterRepository
	tagRepo     storage.ITagRepository
	topicRepo   storage.ITopicRepository
	filter      service.IRelevanceFilter
	coldIndex   storage.IColdIndex
	llm         client.ILLMProvider
	logger      *zap.Logger
}

// ProgressiveQuery implements service.IQueryService.
// Hot path (sequential): (1) optional topic narrowing then LLM-filter catalog tags, (2) documents by tags + LLM relevance,
// (3) chapters for those docs + LLM relevance, (4) buildResults maps chapters to QueryItem (prefers chapter summary, else content).
// Cold path runs in parallel: IColdIndex.Search over cold chapters; results merge with hot by document id.
func (s *queryService) ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	response := &types.ProgressiveQueryResponse{
		Question: req.Question,
		Steps:    []types.ProgressiveQueryStep{},
	}

	var traceIDStr string
	if sp := trace.SpanFromContext(ctx); sp.SpanContext().IsValid() && sp.IsRecording() {
		traceIDStr = sp.SpanContext().TraceID().String()
	}

	wantTrace := common.ProgressiveTraceRequested(ctx)

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

	rootCtx := ctx
	var tracer trace.Tracer

	if wantTrace {
		tracer = otel.Tracer(common.ProgressiveTracerScope)
		var rootSpan trace.Span
		rootCtx, rootSpan = tracer.Start(ctx, "progressive_query",
			trace.WithAttributes(attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, 512))))
		rootCtx = common.WithProgressiveDebugTracer(rootCtx, tracer)
		defer rootSpan.End()

		go func() {
			hCtx, hotSpan := tracer.Start(rootCtx, "hot_path")
			defer hotSpan.End()
			hCtx = common.WithProgressiveDebugTracer(hCtx, tracer)
			results, steps, err := s.queryHotPath(hCtx, req)
			if err != nil {
				hotSpan.RecordError(err)
				hotSpan.SetStatus(codes.Error, err.Error())
			}
			hotChan <- hotResult{results: results, steps: steps, err: err}
		}()

		go func() {
			cCtx, coldSpan := tracer.Start(rootCtx, "cold_path")
			defer coldSpan.End()
			cCtx = common.WithProgressiveDebugTracer(cCtx, tracer)
			results, step, err := s.queryColdPath(cCtx, req)
			if err != nil {
				coldSpan.RecordError(err)
				coldSpan.SetStatus(codes.Error, err.Error())
			}
			coldChan <- coldResult{results: results, step: step, err: err}
		}()
	} else {
		go func() {
			results, steps, err := s.queryHotPath(ctx, req)
			hotChan <- hotResult{results: results, steps: steps, err: err}
		}()
		go func() {
			results, step, err := s.queryColdPath(ctx, req)
			coldChan <- coldResult{results: results, step: step, err: err}
		}()
	}

	hotRes := <-hotChan
	coldRes := <-coldChan

	if hotRes.err != nil {
		s.logger.Error("hot path query failed", zap.Error(hotRes.err))
	} else {
		response.Steps = append(response.Steps, hotRes.steps...)
	}

	if coldRes.err != nil {
		s.logger.Error("cold path query failed", zap.Error(coldRes.err))
	} else {
		response.Steps = append(response.Steps, coldRes.step)
	}

	mergedResults := s.mergeHotAndColdResults(hotRes.results, coldRes.results, req.MaxResults)
	response.Results = mergedResults

	if wantTrace {
		_, mergeSpan := tracer.Start(rootCtx, "merge_results", trace.WithAttributes(
			attribute.Int("tier.request.merge_inputs.hot_items", len(hotRes.results)),
			attribute.Int("tier.request.merge_inputs.cold_items", len(coldRes.results)),
			attribute.Int("tier.response.merged_items", len(mergedResults)),
		))
		mergeSpan.End()

		ansCtx, ansSpan := tracer.Start(rootCtx, "synthesize_answer", trace.WithAttributes(
			attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, common.TraceMaxReqBytes)),
			attribute.Int("tier.request.reference_items", len(mergedResults)),
		))
		ansCtx = trace.ContextWithSpan(ansCtx, ansSpan)
		ansCtx = common.WithProgressiveDebugTracer(ansCtx, tracer)
		response.Answer = s.generateProgressiveAnswer(ansCtx, req.Question, mergedResults)
		if response.Answer != "" {
			ansSpan.SetAttributes(attribute.String("tier.response.answer", common.TruncateTraceStr(response.Answer, common.TraceMaxRespBytes)))
		}
		ansSpan.End()
	} else {
		response.Answer = s.generateProgressiveAnswer(ctx, req.Question, mergedResults)
	}

	if traceIDStr != "" {
		response.TraceID = traceIDStr
	}

	s.logger.Info("progressive query completed",
		zap.String("question", req.Question),
		zap.Int("hot_results", len(hotRes.results)),
		zap.Int("cold_results", len(coldRes.results)),
		zap.Int("total_results", len(mergedResults)),
		zap.Bool("has_answer", response.Answer != ""),
		zap.String("otel_trace_id", traceIDStr),
	)

	return response, nil
}

// queryHotPath performs the hot document query path (tag-based progressive).
func (s *queryService) queryHotPath(ctx context.Context, req types.ProgressiveQueryRequest) ([]types.QueryItem, []types.ProgressiveQueryStep, error) {
	start := time.Now()
	var steps []types.ProgressiveQueryStep

	// Step 1: Filter catalog tags for the question (optionally via topics).
	step1Start := time.Now()
	var tagNames []string
	err := common.WithOptionalSpan(ctx, "filter_tags", func(c context.Context, sp trace.Span) error {
		var e error
		tagNames, e = s.filterCatalogTags(c, req.Question)
		if sp != nil && e == nil {
			sp.SetAttributes(
				attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, common.TraceMaxReqBytes)),
				attribute.Int("tier.response.tags_count", len(tagNames)),
			)
		}
		return e
	})
	if err != nil {
		s.logger.Error("failed to filter catalog tags", zap.Error(err))
		return nil, nil, fmt.Errorf("filter catalog tags: %w", err)
	}
	steps = append(steps, types.ProgressiveQueryStep{
		Step:     "tags",
		Input:    req.Question,
		Output:   tagNames,
		Duration: time.Since(step1Start).Milliseconds(),
	})

	// Step 2: Query documents by tags and filter.
	step2Start := time.Now()
	var docs []types.Document
	err = common.WithOptionalSpan(ctx, "query_and_filter_documents", func(c context.Context, sp trace.Span) error {
		var e error
		docs, e = s.queryAndFilterDocuments(c, req.Question, tagNames, req.MaxResults)
		if sp != nil && e == nil {
			sp.SetAttributes(
				attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, common.TraceMaxReqBytes)),
				attribute.Int("tier.request.max_results", req.MaxResults),
				attribute.Int("tier.response.documents_count", len(docs)),
			)
		}
		return e
	})
	if err != nil {
		s.logger.Error("failed to query documents", zap.Error(err))
		return nil, nil, fmt.Errorf("query documents: %w", err)
	}
	steps = append(steps, types.ProgressiveQueryStep{
		Step:     "documents",
		Input:    tagNames,
		Output:   len(docs),
		Duration: time.Since(step2Start).Milliseconds(),
	})

	// Track document access for hot/cold management.
	s.trackDocumentAccess(ctx, docs)

	// Step 3: Query chapters by docs and filter.
	step3Start := time.Now()
	var chapters []types.Chapter
	err = common.WithOptionalSpan(ctx, "query_and_filter_chapters", func(c context.Context, sp trace.Span) error {
		var e error
		chapters, e = s.queryAndFilterChapters(c, req.Question, docs)
		if sp != nil && e == nil {
			sp.SetAttributes(
				attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, common.TraceMaxReqBytes)),
				attribute.Int("tier.response.chapters_count", len(chapters)),
			)
		}
		return e
	})
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

	// Record metrics.
	metrics.RecordQueryLatency(metrics.QueryPathHot, time.Since(start).Seconds(), len(results))

	return results, steps, nil
}

// queryColdPath performs the cold document query path (cold index search).
func (s *queryService) queryColdPath(ctx context.Context, req types.ProgressiveQueryRequest) ([]types.QueryItem, types.ProgressiveQueryStep, error) {
	start := time.Now()

	if s.coldIndex == nil {
		_ = common.WithOptionalSpan(ctx, "cold_index_search", func(_ context.Context, sp trace.Span) error {
			if sp != nil {
				sp.SetAttributes(
					attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, common.TraceMaxReqBytes)),
					attribute.Bool("tier.response.cold_index_skipped", true),
					attribute.String("tier.response.cold_index_skip_reason", "no_index"),
				)
			}
			return nil
		})
		return nil, types.ProgressiveQueryStep{
			Step:     "cold_docs",
			Input:    req.Question,
			Output:   0,
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}

	var searchResults []storage.ColdIndexHit
	err := common.WithOptionalSpan(ctx, "cold_index_search", func(c context.Context, sp trace.Span) error {
		if sp != nil {
			sp.SetAttributes(
				attribute.String("tier.request.question", common.TruncateTraceStr(req.Question, common.TraceMaxReqBytes)),
				attribute.Int("tier.request.cold_search_max_results", req.MaxResults/2),
			)
		}
		var e error
		searchResults, e = s.coldIndex.Search(c, req.Question, req.MaxResults/2)
		if sp != nil && e == nil {
			sp.SetAttributes(attribute.Int("tier.response.cold_index_hits", len(searchResults)))
		}
		return e
	})
	if err != nil {
		return nil, types.ProgressiveQueryStep{}, fmt.Errorf("cold index search failed: %w", err)
	}

	// Convert search results to query items.
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
			Path:          path,
			Relevance:     sr.Score,
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

// mergeHotAndColdResults merges results from hot and cold paths.
func (s *queryService) mergeHotAndColdResults(hotResults, coldResults []types.QueryItem, maxResults int) []types.QueryItem {
	// Use a map to deduplicate by document ID.
	resultMap := make(map[string]*types.QueryItem)

	// Add hot results first (they have higher quality from LLM filtering).
	for i := range hotResults {
		r := &hotResults[i]
		resultMap[r.ID] = r
	}

	// Add cold results, avoiding duplicates.
	for i := range coldResults {
		r := &coldResults[i]
		if existing, ok := resultMap[r.ID]; ok {
			// Document exists in both - boost relevance if cold result is also found.
			if r.Relevance > existing.Relevance {
				existing.Relevance = r.Relevance
			}
		} else {
			resultMap[r.ID] = r
		}
	}

	// Convert map to slice.
	results := make([]types.QueryItem, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// Sort by relevance descending.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})

	// Return top results.
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// trackDocumentAccess increments query count for accessed documents
// and triggers promotion for cold documents that exceed the threshold.
func (s *queryService) trackDocumentAccess(ctx context.Context, docs []types.Document) {
	for _, doc := range docs {
		// Increment query count in background.
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

// filterCatalogTags lists catalog tags and filters them for the query (adaptive topic narrowing).
func (s *queryService) filterCatalogTags(ctx context.Context, query string) ([]string, error) {
	allTags, err := s.tagRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list catalog tags: %w", err)
	}

	if len(allTags) == 0 {
		return nil, nil
	}

	if len(allTags) < CatalogTagThreshold {
		s.logger.Info("adaptive filtering: direct tag filter (count below threshold)",
			zap.Int("tag_count", len(allTags)),
			zap.Int("threshold", CatalogTagThreshold))
		return s.filterTagsDirect(ctx, query, allTags)
	}

	s.logger.Info("adaptive filtering: topic then tag filter (count at or above threshold)",
		zap.Int("tag_count", len(allTags)),
		zap.Int("threshold", CatalogTagThreshold))
	return s.filterTagsViaTopics(ctx, query)
}

func (s *queryService) filterTagsDirect(ctx context.Context, query string, tags []types.Tag) ([]string, error) {
	type filterer interface {
		FilterTagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error)
	}

	f, ok := s.filter.(filterer)
	if !ok {
		s.logger.Warn("relevance filter does not implement FilterTagsByQuery; returning all catalog tag names")
		return s.extractTagNames(tags), nil
	}

	filterResults, err := f.FilterTagsByQuery(ctx, query, tags)
	if err != nil {
		s.logger.Warn("LLM tag filter failed, using all tags", zap.Error(err))
		return s.extractTagNames(tags), nil
	}

	return s.extractRelevantTags(filterResults), nil
}

func (s *queryService) filterTagsViaTopics(ctx context.Context, query string) ([]string, error) {
	selectedTopics, err := s.filterTopics(ctx, query)
	if err != nil {
		s.logger.Warn("topic filter failed, falling back to direct tag filter", zap.Error(err))
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterTagsDirect(ctx, query, allTags)
	}

	if len(selectedTopics) == 0 {
		s.logger.Warn("no topics selected, falling back to direct tag filter")
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterTagsDirect(ctx, query, allTags)
	}

	s.logger.Info("topics selected", zap.Int("count", len(selectedTopics)))

	topicIDs := make([]string, len(selectedTopics))
	for i, g := range selectedTopics {
		topicIDs[i] = g.ID
	}

	tagsInTopics, err := s.getTagsFromTopics(ctx, topicIDs)
	if err != nil {
		s.logger.Warn("failed to get tags from topics, falling back to direct tag filter", zap.Error(err))
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterTagsDirect(ctx, query, allTags)
	}

	if len(tagsInTopics) == 0 {
		s.logger.Warn("no tags in selected topics, falling back to direct tag filter")
		allTags, _ := s.tagRepo.List(ctx)
		return s.filterTagsDirect(ctx, query, allTags)
	}

	s.logger.Info("tags collected from selected topics", zap.Int("count", len(tagsInTopics)))

	return s.filterTagsDirect(ctx, query, tagsInTopics)
}

func (s *queryService) filterTopics(ctx context.Context, query string) ([]types.Topic, error) {
	topics, err := s.topicRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}

	if len(topics) == 0 {
		return nil, nil
	}

	type topicFilterer interface {
		FilterTopicsByQuery(ctx context.Context, query string, topics []types.Topic) ([]types.LLMFilterResult, error)
	}

	f, ok := s.filter.(topicFilterer)
	if !ok {
		s.logger.Warn("relevance filter does not implement FilterTopicsByQuery; returning all topics")
		return topics, nil
	}

	filterResults, err := f.FilterTopicsByQuery(ctx, query, topics)
	if err != nil {
		s.logger.Warn("LLM topic filter failed, returning all topics", zap.Error(err))
		return topics, nil
	}

	topicMap := make(map[string]types.Topic)
	for _, g := range topics {
		topicMap[g.ID] = g
	}

	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	var selected []types.Topic
	for _, fr := range filterResults {
		if fr.Relevance >= 0.5 && len(selected) < 3 {
			if g, ok := topicMap[fr.ID]; ok {
				selected = append(selected, g)
			}
		}
	}

	return selected, nil
}

func (s *queryService) getTagsFromTopics(ctx context.Context, topicIDs []string) ([]types.Tag, error) {
	var allTags []types.Tag
	seenTags := make(map[string]bool)

	for _, tid := range topicIDs {
		tags, err := s.tagRepo.ListByTopic(ctx, tid)
		if err != nil {
			s.logger.Warn("failed to get tags by topic", zap.String("topic_id", tid), zap.Error(err))
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

// extractTagNames extracts tag names from tags.
func (s *queryService) extractTagNames(tags []types.Tag) []string {
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = tag.Name
	}
	return names
}

// extractRelevantTags extracts tag names with relevance >= 0.5 from filter results.
func (s *queryService) extractRelevantTags(results []types.TagFilterResult) []string {
	var names []string
	for _, r := range results {
		if r.Relevance >= 0.5 {
			names = append(names, r.Tag)
		}
	}
	return names
}

// queryAndFilterDocuments queries documents by tags and filters by query.
// For hot docs: uses LLM filtering.
// For cold docs: uses simple keyword matching.
func (s *queryService) queryAndFilterDocuments(ctx context.Context, query string, tags []string, limit int) ([]types.Document, error) {
	var docs []types.Document
	var err error

	if len(tags) == 0 {
		// No tag names after filtering (empty catalog, or LLM returned no tags above threshold).
		// Scope documents by listing up to limit; hot/cold paths still apply LLM or keyword filtering.
		s.logger.Debug("progressive query: no tag filter results; listing documents as fallback",
			zap.Int("limit", limit))
		docs, err = s.docRepo.ListAll(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("list all documents: %w", err)
		}
	} else {
		// Query documents by tags (OR logic - documents matching ANY tag).
		docs, err = s.docRepo.ListByTags(ctx, tags, limit)
		if err != nil {
			return nil, fmt.Errorf("list documents by tags: %w", err)
		}
	}

	if len(docs) == 0 {
		return nil, nil
	}

	// Separate hot and cold documents.
	var hotDocs, coldDocs []types.Document
	for _, doc := range docs {
		if doc.Status == types.DocStatusHot {
			hotDocs = append(hotDocs, doc)
		} else {
			coldDocs = append(coldDocs, doc)
		}
	}

	var filteredDocs []types.Document

	// For hot documents: use LLM filtering.
	if len(hotDocs) > 0 {
		hotFiltered, err := s.filterHotDocuments(ctx, query, hotDocs)
		if err != nil {
			s.logger.Warn("LLM document filter failed for hot docs", zap.Error(err))
			filteredDocs = append(filteredDocs, hotDocs...)
		} else {
			filteredDocs = append(filteredDocs, hotFiltered...)
		}
	}

	// For cold documents: use simple keyword matching.
	if len(coldDocs) > 0 {
		coldFiltered := s.filterColdDocuments(query, coldDocs)
		filteredDocs = append(filteredDocs, coldFiltered...)
	}

	return filteredDocs, nil
}

// filterHotDocuments filters hot documents using LLM.
func (s *queryService) filterHotDocuments(ctx context.Context, query string, docs []types.Document) ([]types.Document, error) {
	filterResults, err := s.filter.FilterDocuments(ctx, query, docs)
	if err != nil {
		return nil, err
	}

	// Build doc map for lookup.
	docMap := make(map[string]types.Document)
	for _, doc := range docs {
		docMap[doc.ID] = doc
	}

	// Sort by relevance and filter.
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

// filterColdDocuments filters cold documents using simple keyword matching.
func (s *queryService) filterColdDocuments(query string, docs []types.Document) []types.Document {
	keywords := types.ExtractKeywords(query, 10)
	var filteredDocs []types.Document

	for _, doc := range docs {
		if s.matchesColdDocument(doc, keywords) {
			filteredDocs = append(filteredDocs, doc)
		}
	}

	return filteredDocs
}

// matchesColdDocument checks if a cold document matches the query keywords.
func (s *queryService) matchesColdDocument(doc types.Document, keywords []string) bool {
	contentLower := strings.ToLower(doc.Content)
	titleLower := strings.ToLower(doc.Title)

	// Match if any keyword is found in title or content.
	for _, keyword := range keywords {
		keywordLower := strings.ToLower(keyword)
		if strings.Contains(titleLower, keywordLower) || strings.Contains(contentLower, keywordLower) {
			return true
		}
	}

	// Also check document tags.
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

// queryAndFilterChapters queries chapters by documents and filters by query.
func (s *queryService) queryAndFilterChapters(ctx context.Context, query string, docs []types.Document) ([]types.Chapter, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Get all chapters for these documents.
	var allChapters []types.Chapter
	for _, doc := range docs {
		// For cold documents, use the full body as one pseudo-chapter for LLM filtering.
		if doc.Status != types.DocStatusHot {
			chapter := s.createColdDocumentChapter(doc, query)
			if chapter != nil {
				allChapters = append(allChapters, *chapter)
			}
			continue
		}

		// For hot documents, get chapters from chapter repository.
		chapters, err := s.chapterRepo.ListByDocument(ctx, doc.ID)
		if err != nil {
			s.logger.Warn("failed to get document chapters", zap.String("doc_id", doc.ID), zap.Error(err))
			continue
		}
		allChapters = append(allChapters, chapters...)
	}

	if len(allChapters) == 0 {
		return nil, nil
	}

	// Filter chapters by query using LLM.
	filterResults, err := s.filter.FilterChapters(ctx, query, allChapters)
	if err != nil {
		s.logger.Warn("LLM chapter filter failed, returning all chapters", zap.Error(err))
		return allChapters, nil
	}

	// Build chapter map.
	chapterMap := make(map[string]types.Chapter)
	for _, ch := range allChapters {
		chapterMap[ch.Path] = ch
	}

	// Sort by relevance and filter.
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	var filteredChapters []types.Chapter
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
func (s *queryService) createColdDocumentChapter(doc types.Document, query string) *types.Chapter {
	_ = query
	return &types.Chapter{
		ID:         doc.ID + "_cold",
		DocumentID: doc.ID,
		Path:       doc.ID + "/full",
		Title:      doc.Title,
		Summary:    doc.Content,
		Content:    doc.Content,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}
}

// buildResults maps filtered chapters to QueryItem rows (Content uses chapter Summary when non-empty, else Content).
func (s *queryService) buildResults(chapters []types.Chapter, docStatusByID map[string]types.DocumentStatus) []types.QueryItem {
	results := make([]types.QueryItem, len(chapters))
	for i, ch := range chapters {
		st := docStatusByID[ch.DocumentID]
		body := ch.Summary
		if strings.TrimSpace(body) == "" {
			body = ch.Content
		}
		results[i] = types.QueryItem{
			ID:            ch.DocumentID,
			Title:         ch.Title,
			Content:       body,
			Path:          ch.Path,
			Relevance:     1.0, // Already filtered by LLM.
			Status:        st,
			ContentSource: "chapter_summary",
		}
	}
	return results
}

// extractTitleFromPath extracts a readable title from path.
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

// splitPath splits path by "/".
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

// splitByChar splits string by character.
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

var _ service.IQueryService = (*queryService)(nil)
