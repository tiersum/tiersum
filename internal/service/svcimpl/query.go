package svcimpl

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// QuerySvc implements service.IQueryService
type QuerySvc struct {
	docRepo      storage.IDocumentRepository
	summaryRepo  storage.ISummaryRepository
	topicRepo    storage.ITopicSummaryRepository
	llmFilter    service.ILLMFilter
	logger       *zap.Logger
}

// NewQuerySvc creates a new query service
func NewQuerySvc(docRepo storage.IDocumentRepository, summaryRepo storage.ISummaryRepository, topicRepo storage.ITopicSummaryRepository, llmFilter service.ILLMFilter, logger *zap.Logger) *QuerySvc {
	return &QuerySvc{
		docRepo:     docRepo,
		summaryRepo: summaryRepo,
		topicRepo:   topicRepo,
		llmFilter:   llmFilter,
		logger:      logger,
	}
}

// Query implements IQueryService.Query
// For backward compatibility - performs a simple hierarchical query
func (s *QuerySvc) Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error) {
	req := types.HierarchicalQueryRequest{
		Question:   question,
		StartTier:  types.TierDocument,
		EndTier:    depth,
		MaxResults: 10,
	}

	resp, err := s.HierarchicalQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to legacy format
	var results []types.QueryResult
	for _, level := range resp.Levels {
		for _, item := range level.Items {
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

// HierarchicalQuery performs progressive hierarchical query with LLM filtering at each level
func (s *QuerySvc) HierarchicalQuery(ctx context.Context, req types.HierarchicalQueryRequest) (*types.HierarchicalQueryResponse, error) {
	// Set defaults
	if req.StartTier == "" {
		req.StartTier = types.TierTopic
	}
	if req.EndTier == "" {
		req.EndTier = types.TierSource
	}
	if req.MaxResults == 0 {
		req.MaxResults = 10
	}

	response := &types.HierarchicalQueryResponse{
		Question: req.Question,
		Levels:   []types.HierarchicalQueryLevel{},
	}

	// Track current state for progressive drill-down
	currentTier := req.StartTier
	var selectedTopicIDs []string
	var selectedDocIDs []string
	var selectedChapterPaths []string

	for {
		level := types.HierarchicalQueryLevel{
			Tier: currentTier,
		}

		switch currentTier {
		case types.TierTopic:
			items, topicIDs, err := s.queryTopicLevel(ctx, req)
			if err != nil {
				return nil, err
			}
			level.Items = items
			level.HasMore = len(items) > 0 && req.EndTier != types.TierTopic
			selectedTopicIDs = topicIDs

		case types.TierDocument:
			items, docIDs, err := s.queryDocumentLevel(ctx, req, selectedTopicIDs)
			if err != nil {
				return nil, err
			}
			level.Items = items
			level.HasMore = len(items) > 0 && req.EndTier != types.TierDocument
			selectedDocIDs = docIDs

		case types.TierChapter:
			items, chapterPaths, err := s.queryChapterLevel(ctx, req, selectedDocIDs)
			if err != nil {
				return nil, err
			}
			level.Items = items
			level.HasMore = len(items) > 0 && req.EndTier != types.TierChapter
			selectedChapterPaths = chapterPaths

		case types.TierParagraph:
			items, err := s.queryParagraphLevel(ctx, req, selectedChapterPaths)
			if err != nil {
				return nil, err
			}
			level.Items = items
			level.HasMore = len(items) > 0 && req.EndTier != types.TierParagraph

		case types.TierSource:
			items, err := s.querySourceLevel(ctx, selectedChapterPaths)
			if err != nil {
				return nil, err
			}
			level.Items = items
			level.HasMore = false
		}

		response.Levels = append(response.Levels, level)

		// Check if we should stop
		if currentTier == req.EndTier || !level.HasMore {
			break
		}

		// Move to next tier
		currentTier = s.getNextTier(currentTier)
		if currentTier == "" {
			break
		}
	}

	return response, nil
}

// queryTopicLevel queries and filters topics
func (s *QuerySvc) queryTopicLevel(ctx context.Context, req types.HierarchicalQueryRequest) ([]types.QueryItem, []string, error) {
	// Get all topics
	topics, err := s.topicRepo.List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list topics: %w", err)
	}

	if len(topics) == 0 {
		return nil, nil, nil
	}

	// Filter by tags if specified
	if len(req.Tags) > 0 {
		filteredTopics, err := s.topicRepo.FindByTags(ctx, req.Tags)
		if err != nil {
			return nil, nil, fmt.Errorf("filter topics by tags: %w", err)
		}
		topics = filteredTopics
	}

	// LLM filter
	filterResults, err := s.llmFilter.FilterTopics(ctx, req.Question, topics)
	if err != nil {
		s.logger.Warn("LLM topic filter failed, returning all topics", zap.Error(err))
		// Fallback: return all topics
		filterResults = make([]types.LLMFilterResult, len(topics))
		for i, t := range topics {
			filterResults[i] = types.LLMFilterResult{ID: t.ID, Relevance: 0.5}
		}
	}

	// Sort by relevance
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	// Build result items
	var items []types.QueryItem
	var topicIDs []string
	topicMap := make(map[string]types.TopicSummary)
	for _, t := range topics {
		topicMap[t.ID] = t
	}

	for _, fr := range filterResults {
		if len(items) >= req.MaxResults {
			break
		}

		topic, ok := topicMap[fr.ID]
		if !ok {
			continue
		}

		items = append(items, types.QueryItem{
			ID:         topic.ID,
			Title:      topic.Name,
			Content:    topic.Summary,
			Tier:       types.TierTopic,
			Path:       "",
			Relevance:  fr.Relevance,
			IsSource:   false, // Topic can drill down to documents
			ChildCount: len(topic.DocumentIDs),
		})
		topicIDs = append(topicIDs, topic.ID)
	}

	return items, topicIDs, nil
}

// queryDocumentLevel queries and filters documents
func (s *QuerySvc) queryDocumentLevel(ctx context.Context, req types.HierarchicalQueryRequest, topicIDs []string) ([]types.QueryItem, []string, error) {
	var docs []types.Document

	// If we have selected topics, get their documents
	if len(topicIDs) > 0 {
		for _, tid := range topicIDs {
			topicDocs, err := s.topicRepo.GetTopicDocuments(ctx, tid)
			if err != nil {
				s.logger.Warn("failed to get topic documents", zap.String("topic_id", tid), zap.Error(err))
				continue
			}
			docs = append(docs, topicDocs...)
		}

		// Deduplicate
		docMap := make(map[string]types.Document)
		for _, d := range docs {
			docMap[d.ID] = d
		}
		docs = make([]types.Document, 0, len(docMap))
		for _, d := range docMap {
			docs = append(docs, d)
		}
	} else {
		// No topics selected, this shouldn't happen normally
		s.logger.Warn("no topics selected for document query")
		return nil, nil, nil
	}

	if len(docs) == 0 {
		return nil, nil, nil
	}

	// LLM filter
	filterResults, err := s.llmFilter.FilterDocuments(ctx, req.Question, docs)
	if err != nil {
		s.logger.Warn("LLM document filter failed, returning all documents", zap.Error(err))
		filterResults = make([]types.LLMFilterResult, len(docs))
		for i, d := range docs {
			filterResults[i] = types.LLMFilterResult{ID: d.ID, Relevance: 0.5}
		}
	}

	// Sort by relevance
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	// Build result items
	var items []types.QueryItem
	var docIDs []string
	docMap := make(map[string]types.Document)
	for _, d := range docs {
		docMap[d.ID] = d
	}

	for _, fr := range filterResults {
		if len(items) >= req.MaxResults {
			break
		}

		doc, ok := docMap[fr.ID]
		if !ok {
			continue
		}

		// Get document summary
		summaries, _ := s.summaryRepo.GetByDocument(ctx, doc.ID)
		var docSummary string
		for _, sum := range summaries {
			if sum.Tier == types.TierDocument {
				docSummary = sum.Content
				break
			}
		}

		// Count chapters
		chapters, _ := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, doc.ID)

		items = append(items, types.QueryItem{
			ID:         doc.ID,
			Title:      doc.Title,
			Content:    docSummary,
			Tier:       types.TierDocument,
			Path:       doc.ID,
			Relevance:  fr.Relevance,
			IsSource:   len(chapters) == 0, // No chapters means cannot drill down
			ChildCount: len(chapters),
		})
		docIDs = append(docIDs, doc.ID)
	}

	return items, docIDs, nil
}

// queryChapterLevel queries and filters chapters
func (s *QuerySvc) queryChapterLevel(ctx context.Context, req types.HierarchicalQueryRequest, docIDs []string) ([]types.QueryItem, []string, error) {
	if len(docIDs) == 0 {
		return nil, nil, nil
	}

	// Get all chapters for selected documents
	var allChapters []types.Summary
	for _, docID := range docIDs {
		chapters, err := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, docID)
		if err != nil {
			s.logger.Warn("failed to get chapters", zap.String("doc_id", docID), zap.Error(err))
			continue
		}
		allChapters = append(allChapters, chapters...)
	}

	if len(allChapters) == 0 {
		return nil, nil, nil
	}

	// Separate short chapters (is_source=true) and normal chapters
	var normalChapters []types.Summary
	var shortChapterItems []types.QueryItem

	for _, ch := range allChapters {
		if ch.IsSource {
			// Short chapter - include directly with max relevance
			shortChapterItems = append(shortChapterItems, types.QueryItem{
			ID:         ch.DocumentID,
			Title:      extractTitleFromPath(ch.Path),
			Content:    ch.Content, // Already source
			Tier:       types.TierChapter,
			Path:       ch.Path,
			Relevance:  1.0, // Max relevance for short chapters
			IsSource:   true, // Short chapter is source, cannot drill down
			ChildCount: 0,
		})
		} else {
			normalChapters = append(normalChapters, ch)
		}
	}

	// LLM filter normal chapters
	var filteredNormalItems []types.QueryItem
	if len(normalChapters) > 0 {
		filterResults, err := s.llmFilter.FilterSummaries(ctx, req.Question, normalChapters, types.TierChapter)
		if err != nil {
			s.logger.Warn("LLM chapter filter failed", zap.Error(err))
			// Fallback: include all normal chapters
			filterResults = make([]types.LLMFilterResult, len(normalChapters))
			for i, ch := range normalChapters {
				filterResults[i] = types.LLMFilterResult{ID: ch.Path, Relevance: 0.5}
			}
		}

		// Sort by relevance
		sort.Slice(filterResults, func(i, j int) bool {
			return filterResults[i].Relevance > filterResults[j].Relevance
		})

		// Build chapter map
		chapterMap := make(map[string]types.Summary)
		for _, ch := range normalChapters {
			chapterMap[ch.Path] = ch
		}

		// Build result items for normal chapters
		for _, fr := range filterResults {
			ch, ok := chapterMap[fr.ID]
			if !ok {
				continue
			}

			// Count paragraphs
			paragraphs, _ := s.summaryRepo.GetChildrenPaths(ctx, ch.Path, types.TierParagraph)

			filteredNormalItems = append(filteredNormalItems, types.QueryItem{
				ID:         ch.DocumentID,
				Title:      extractTitleFromPath(ch.Path),
				Content:    ch.Content,
				Tier:       types.TierChapter,
				Path:       ch.Path,
				Relevance:  fr.Relevance,
				IsSource:   len(paragraphs) == 0, // No paragraphs means cannot drill down
				ChildCount: len(paragraphs),
			})
		}
	}

	// Combine: short chapters first (they're already source), then filtered normal chapters
	var items []types.QueryItem
	var chapterPaths []string

	// Add short chapters (already source)
	for _, item := range shortChapterItems {
		if len(items) >= req.MaxResults {
			break
		}
		items = append(items, item)
		chapterPaths = append(chapterPaths, item.Path)
	}

	// Add filtered normal chapters
	for _, item := range filteredNormalItems {
		if len(items) >= req.MaxResults {
			break
		}
		items = append(items, item)
		chapterPaths = append(chapterPaths, item.Path)
	}

	return items, chapterPaths, nil
}

// queryParagraphLevel queries and filters paragraphs
func (s *QuerySvc) queryParagraphLevel(ctx context.Context, req types.HierarchicalQueryRequest, chapterPaths []string) ([]types.QueryItem, error) {
	if len(chapterPaths) == 0 {
		return nil, nil
	}

	// Get all paragraphs for selected chapters
	var allParagraphs []types.Summary
	for _, chPath := range chapterPaths {
		// Skip if chapter is marked as source (no paragraphs)
		ch, err := s.summaryRepo.GetByPath(ctx, chPath)
		if err != nil || ch == nil || ch.IsSource {
			continue
		}

		paragraphs, err := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierParagraph, chPath)
		if err != nil {
			s.logger.Warn("failed to get paragraphs", zap.String("chapter_path", chPath), zap.Error(err))
			continue
		}
		allParagraphs = append(allParagraphs, paragraphs...)
	}

	if len(allParagraphs) == 0 {
		return nil, nil
	}

	// LLM filter
	filterResults, err := s.llmFilter.FilterSummaries(ctx, req.Question, allParagraphs, types.TierParagraph)
	if err != nil {
		s.logger.Warn("LLM paragraph filter failed", zap.Error(err))
		filterResults = make([]types.LLMFilterResult, len(allParagraphs))
		for i, p := range allParagraphs {
			filterResults[i] = types.LLMFilterResult{ID: p.Path, Relevance: 0.5}
		}
	}

	// Sort by relevance
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	// Build paragraph map
	paraMap := make(map[string]types.Summary)
	for _, p := range allParagraphs {
		paraMap[p.Path] = p
	}

	// Build result items
	var items []types.QueryItem
	for _, fr := range filterResults {
		if len(items) >= req.MaxResults {
			break
		}

		para, ok := paraMap[fr.ID]
		if !ok {
			continue
		}

		items = append(items, types.QueryItem{
			ID:         para.DocumentID,
			Title:      fmt.Sprintf("Paragraph %s", extractLastSegment(para.Path)),
			Content:    para.Content,
			Tier:       types.TierParagraph,
			Path:       para.Path,
			Relevance:  fr.Relevance,
			IsSource:   false, // Paragraph can drill down to source
			ChildCount: 1,
		})
	}

	return items, nil
}

// querySourceLevel retrieves original source content
func (s *QuerySvc) querySourceLevel(ctx context.Context, paths []string) ([]types.QueryItem, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	var items []types.QueryItem

	for _, path := range paths {
		// Get the summary at this path
		summary, err := s.summaryRepo.GetByPath(ctx, path)
		if err != nil {
			s.logger.Warn("failed to get summary for source", zap.String("path", path), zap.Error(err))
			continue
		}
		if summary == nil {
			continue
		}

		// If it's already marked as source, return its content
		if summary.IsSource {
			items = append(items, types.QueryItem{
				ID:         summary.DocumentID,
				Title:      extractTitleFromPath(summary.Path),
				Content:    summary.Content,
				Tier:       types.TierSource,
				Path:       summary.Path,
				Relevance:  1.0,
				IsSource:   true, // Source is final level
				ChildCount: 0,
			})
			continue
		}

		// For paragraphs, we need to extract from original document
		if summary.Tier == types.TierParagraph {
			doc, err := s.docRepo.GetByID(ctx, summary.DocumentID)
			if err != nil || doc == nil {
				s.logger.Warn("failed to get document for source extraction", zap.String("doc_id", summary.DocumentID))
				continue
			}

			// Extract source from document based on path
			sourceContent := s.extractSourceFromDocument(doc.Content, summary.Path)

			items = append(items, types.QueryItem{
				ID:         summary.DocumentID,
				Title:      fmt.Sprintf("Source: %s", extractTitleFromPath(summary.Path)),
				Content:    sourceContent,
				Tier:       types.TierSource,
				Path:       summary.Path,
				Relevance:  1.0,
				IsSource:   true, // Source is final level
				ChildCount: 0,
			})
		}
	}

	return items, nil
}

// extractSourceFromDocument extracts source content from document based on path
func (s *QuerySvc) extractSourceFromDocument(content string, path string) string {
	// Path format: doc_id/chapter_title/paragraph_index
	// For now, return truncated content if too long
	// TODO: Implement proper offset-based extraction

	if len(content) > 2000 {
		return content[:2000] + "\n\n[Content truncated...]"
	}
	return content
}

// getNextTier returns the next tier in the hierarchy
func (s *QuerySvc) getNextTier(current types.SummaryTier) types.SummaryTier {
	switch current {
	case types.TierTopic:
		return types.TierDocument
	case types.TierDocument:
		return types.TierChapter
	case types.TierChapter:
		return types.TierParagraph
	case types.TierParagraph:
		return types.TierSource
	default:
		return ""
	}
}

// extractTitleFromPath extracts a readable title from path
func extractTitleFromPath(path string) string {
	// Path format: doc_id/chapter_title/paragraph_index
	parts := splitPath(path)
	if len(parts) == 0 {
		return path
	}
	if len(parts) == 1 {
		return parts[0]
	}
	// Return last meaningful part
	lastPart := parts[len(parts)-1]
	if isNumber(lastPart) && len(parts) > 1 {
		return fmt.Sprintf("%s (Section %s)", parts[len(parts)-2], lastPart)
	}
	return lastPart
}

// extractLastSegment extracts the last segment of a path
func extractLastSegment(path string) string {
	parts := splitPath(path)
	if len(parts) == 0 {
		return path
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

// isNumber checks if string is a number
func isNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// GetTopicDocuments retrieves all documents under a topic
func (s *QuerySvc) GetTopicDocuments(ctx context.Context, topicID string) ([]types.Document, error) {
	return s.topicRepo.GetTopicDocuments(ctx, topicID)
}

// GetDocumentChapters retrieves all chapters of a document
func (s *QuerySvc) GetDocumentChapters(ctx context.Context, docID string) ([]types.Summary, error) {
	return s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, docID)
}

// GetChapterParagraphs retrieves all paragraphs under a chapter
func (s *QuerySvc) GetChapterParagraphs(ctx context.Context, chapterPath string) ([]types.Summary, error) {
	return s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierParagraph, chapterPath)
}

// DrillDown performs a drill-down query from current level to next
func (s *QuerySvc) DrillDown(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error) {
	switch req.CurrentTier {
	case types.TierTopic:
		return s.drillDownFromTopic(ctx, req)
	case types.TierDocument:
		return s.drillDownFromDocument(ctx, req)
	case types.TierChapter:
		return s.drillDownFromChapter(ctx, req)
	case types.TierParagraph:
		return s.drillDownFromParagraph(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported tier for drill-down: %s", req.CurrentTier)
	}
}

// drillDownFromTopic drills down from topic to documents
func (s *QuerySvc) drillDownFromTopic(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error) {
	// Get documents under this topic
	docs, err := s.topicRepo.GetTopicDocuments(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get topic documents: %w", err)
	}
	if len(docs) == 0 {
		return nil, nil
	}

	// LLM filter documents by question
	filterResults, err := s.llmFilter.FilterDocuments(ctx, req.Question, docs)
	if err != nil {
		s.logger.Warn("LLM document filter failed, returning all documents", zap.Error(err))
		filterResults = make([]types.LLMFilterResult, len(docs))
		for i, d := range docs {
			filterResults[i] = types.LLMFilterResult{ID: d.ID, Relevance: 0.5}
		}
	}

	// Sort by relevance
	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	// Build result items
	var items []types.QueryItem
	docMap := make(map[string]types.Document)
	for _, d := range docs {
		docMap[d.ID] = d
	}

	for _, fr := range filterResults {
		doc, ok := docMap[fr.ID]
		if !ok {
			continue
		}

		// Get document summary
		summaries, _ := s.summaryRepo.GetByDocument(ctx, doc.ID)
		var docSummary string
		for _, sum := range summaries {
			if sum.Tier == types.TierDocument {
				docSummary = sum.Content
				break
			}
		}

		// Count chapters
		chapters, _ := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, doc.ID)

		items = append(items, types.QueryItem{
			ID:         doc.ID,
			Title:      doc.Title,
			Content:    docSummary,
			Tier:       types.TierDocument,
			Path:       doc.ID,
			Relevance:  fr.Relevance,
			IsSource:   len(chapters) == 0,
			ChildCount: len(chapters),
		})
	}

	return items, nil
}

// drillDownFromDocument drills down from document to chapters
func (s *QuerySvc) drillDownFromDocument(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error) {
	// Get all chapters for this document
	chapters, err := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document chapters: %w", err)
	}
	if len(chapters) == 0 {
		return nil, nil
	}

	// Separate short chapters (is_source=true) and normal chapters
	var normalChapters []types.Summary
	var items []types.QueryItem

	for _, ch := range chapters {
		if ch.IsSource {
			// Short chapter - include directly
			items = append(items, types.QueryItem{
				ID:         ch.DocumentID,
				Title:      extractTitleFromPath(ch.Path),
				Content:    ch.Content,
				Tier:       types.TierChapter,
				Path:       ch.Path,
				Relevance:  1.0,
				IsSource:   true,
				ChildCount: 0,
			})
		} else {
			normalChapters = append(normalChapters, ch)
		}
	}

	// LLM filter normal chapters
	if len(normalChapters) > 0 {
		filterResults, err := s.llmFilter.FilterSummaries(ctx, req.Question, normalChapters, types.TierChapter)
		if err != nil {
			s.logger.Warn("LLM chapter filter failed", zap.Error(err))
			filterResults = make([]types.LLMFilterResult, len(normalChapters))
			for i, ch := range normalChapters {
				filterResults[i] = types.LLMFilterResult{ID: ch.Path, Relevance: 0.5}
			}
		}

		sort.Slice(filterResults, func(i, j int) bool {
			return filterResults[i].Relevance > filterResults[j].Relevance
		})

		chapterMap := make(map[string]types.Summary)
		for _, ch := range normalChapters {
			chapterMap[ch.Path] = ch
		}

		for _, fr := range filterResults {
			ch, ok := chapterMap[fr.ID]
			if !ok {
				continue
			}

			paragraphs, _ := s.summaryRepo.GetChildrenPaths(ctx, ch.Path, types.TierParagraph)

			items = append(items, types.QueryItem{
				ID:         ch.DocumentID,
				Title:      extractTitleFromPath(ch.Path),
				Content:    ch.Content,
				Tier:       types.TierChapter,
				Path:       ch.Path,
				Relevance:  fr.Relevance,
				IsSource:   len(paragraphs) == 0,
				ChildCount: len(paragraphs),
			})
		}
	}

	return items, nil
}

// drillDownFromChapter drills down from chapter to paragraphs
func (s *QuerySvc) drillDownFromChapter(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error) {
	// Get all paragraphs for this chapter
	paragraphs, err := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierParagraph, req.Path)
	if err != nil {
		return nil, fmt.Errorf("get chapter paragraphs: %w", err)
	}
	if len(paragraphs) == 0 {
		return nil, nil
	}

	// LLM filter paragraphs
	filterResults, err := s.llmFilter.FilterSummaries(ctx, req.Question, paragraphs, types.TierParagraph)
	if err != nil {
		s.logger.Warn("LLM paragraph filter failed", zap.Error(err))
		filterResults = make([]types.LLMFilterResult, len(paragraphs))
		for i, p := range paragraphs {
			filterResults[i] = types.LLMFilterResult{ID: p.Path, Relevance: 0.5}
		}
	}

	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	paraMap := make(map[string]types.Summary)
	for _, p := range paragraphs {
		paraMap[p.Path] = p
	}

	var items []types.QueryItem
	for _, fr := range filterResults {
		para, ok := paraMap[fr.ID]
		if !ok {
			continue
		}

		items = append(items, types.QueryItem{
			ID:         para.DocumentID,
			Title:      fmt.Sprintf("Paragraph %s", extractLastSegment(para.Path)),
			Content:    para.Content,
			Tier:       types.TierParagraph,
			Path:       para.Path,
			Relevance:  fr.Relevance,
			IsSource:   false,
			ChildCount: 1,
		})
	}

	return items, nil
}

// drillDownFromParagraph drills down from paragraph to source
func (s *QuerySvc) drillDownFromParagraph(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error) {
	item, err := s.GetSource(ctx, req.DocumentID, req.Path)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return []types.QueryItem{*item}, nil
}

// GetSource retrieves the original source content for a path
func (s *QuerySvc) GetSource(ctx context.Context, docID string, path string) (*types.QueryItem, error) {
	// Get the summary at this path
	summary, err := s.summaryRepo.GetByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}
	if summary == nil {
		return nil, nil
	}

	// If it's already marked as source, return its content
	if summary.IsSource {
		return &types.QueryItem{
			ID:         summary.DocumentID,
			Title:      extractTitleFromPath(summary.Path),
			Content:    summary.Content,
			Tier:       types.TierSource,
			Path:       summary.Path,
			Relevance:  1.0,
			IsSource:   true,
			ChildCount: 0,
		}, nil
	}

	// For paragraphs, extract from original document
	if summary.Tier == types.TierParagraph {
		doc, err := s.docRepo.GetByID(ctx, summary.DocumentID)
		if err != nil || doc == nil {
			s.logger.Warn("failed to get document for source extraction", zap.String("doc_id", summary.DocumentID))
			return nil, nil
		}

		sourceContent := s.extractSourceFromDocument(doc.Content, summary.Path)

		return &types.QueryItem{
			ID:         summary.DocumentID,
			Title:      fmt.Sprintf("Source: %s", extractTitleFromPath(summary.Path)),
			Content:    sourceContent,
			Tier:       types.TierSource,
			Path:       summary.Path,
			Relevance:  1.0,
			IsSource:   true,
			ChildCount: 0,
		}, nil
	}

	return nil, fmt.Errorf("cannot get source for tier: %s", summary.Tier)
}

var _ service.IQueryService = (*QuerySvc)(nil)
var _ service.IHierarchicalQueryService = (*QuerySvc)(nil)
