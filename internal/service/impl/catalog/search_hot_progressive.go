package catalog

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

// catalogTagThreshold controls adaptive topic→tag filtering (same semantics as legacy progressive query).
// Below threshold: LLM filters all catalog tags directly (skip topics). At or above: LLM picks topics first, then tags within those topics.
const catalogTagThreshold = 200

// searchHotChaptersProgressive mirrors the legacy hot progressive pipeline: catalog tags (optional topics) → documents → chapters, each hop LLM-filtered where applicable.
func (s *chapterService) searchHotChaptersProgressive(ctx context.Context, query string, limit int) ([]types.HotSearchHit, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "searchHotChaptersProgressive", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("query", query))
	span.SetAttributes(attribute.Int("limit", limit))

	if limit <= 0 {
		limit = 50
	}
	if s.relCore == nil {
		if s.logger != nil {
			s.logger.Warn("SearchHotChapters: LLM unavailable; returning no hot hits")
		}
		span.SetAttributes(attribute.String("skip_reason", "llm_unavailable"))
		return nil, nil
	}

	docCap := limit
	if docCap < 100 {
		docCap = 100
	}

	tagNames, err := s.filterCatalogTags(ctx, query)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("tag_count", len(tagNames)))
	if len(tagNames) > 0 {
		span.SetAttributes(attribute.String("tags", joinFirstN(tagNames, 10)))
	}

	docs, err := s.queryAndFilterDocumentsForHotSearch(ctx, query, tagNames, docCap)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("filtered_doc_count", len(docs)))
	if len(docs) > 0 {
		ids := make([]string, 0, min(10, len(docs)))
		for i := 0; i < min(10, len(docs)); i++ {
			ids = append(ids, docs[i].ID)
		}
		span.SetAttributes(attribute.String("filtered_doc_ids", joinFirstN(ids, 10)))
	}

	chapters, relByPath, err := s.queryAndFilterChaptersForHotSearch(ctx, query, docs)
	if err != nil {
		return nil, err
	}

	statusByID := make(map[string]types.DocumentStatus, len(docs))
	queryCountByID := make(map[string]int, len(docs))
	for _, d := range docs {
		statusByID[d.ID] = d.Status
		queryCountByID[d.ID] = d.QueryCount
	}

	hits := make([]types.HotSearchHit, 0, len(chapters))
	for _, ch := range chapters {
		body := strings.TrimSpace(ch.Summary)
		if body == "" {
			body = strings.TrimSpace(ch.Content)
		}
		if body == "" {
			continue
		}
		sc := relByPath[ch.Path]
		if sc <= 0 {
			sc = 0.5
		}
		hits = append(hits, types.HotSearchHit{
			DocumentID: ch.DocumentID,
			Path:       ch.Path,
			Title:      ch.Title,
			Content:    body,
			Score:      sc,
			Source:     "hot_progressive",
			Status:     statusByID[ch.DocumentID],
			QueryCount: queryCountByID[ch.DocumentID],
		})
	}

	if len(hits) > limit {
		hits = hits[:limit]
	}
	span.SetAttributes(attribute.Int("hit_count", len(hits)))
	return hits, nil
}

func (s *chapterService) filterCatalogTags(ctx context.Context, query string) ([]string, error) {
	if s.tagRepo == nil {
		return nil, nil
	}
	allTags, err := s.tagRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list catalog tags: %w", err)
	}
	if len(allTags) == 0 {
		return nil, nil
	}

	if len(allTags) < catalogTagThreshold {
		if s.logger != nil {
			s.logger.Info("adaptive filtering: direct tag filter (count below threshold)",
				zap.Int("tag_count", len(allTags)),
				zap.Int("threshold", catalogTagThreshold))
		}
		return s.filterTagsDirect(ctx, query, allTags)
	}

	if s.logger != nil {
		s.logger.Info("adaptive filtering: topic then tag filter (count at or above threshold)",
			zap.Int("tag_count", len(allTags)),
			zap.Int("threshold", catalogTagThreshold))
	}
	return s.filterTagsViaTopics(ctx, query)
}

func (s *chapterService) filterTagsDirect(ctx context.Context, query string, tags []types.Tag) ([]string, error) {
	filterResults, err := s.relCore.FilterTagsByQuery(ctx, query, tags)
	if err != nil {
		return nil, err
	}
	return extractRelevantTags(filterResults), nil
}

func (s *chapterService) filterTagsViaTopics(ctx context.Context, query string) ([]string, error) {
	selectedTopics, err := s.filterTopics(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(selectedTopics) == 0 {
		return nil, nil
	}

	if s.logger != nil {
		s.logger.Info("topics selected", zap.Int("count", len(selectedTopics)))
	}

	topicIDs := make([]string, len(selectedTopics))
	for i, g := range selectedTopics {
		topicIDs[i] = g.ID
	}

	tagsInTopics, err := s.getTagsFromTopics(ctx, topicIDs)
	if err != nil {
		return nil, err
	}

	if len(tagsInTopics) == 0 {
		return nil, nil
	}

	if s.logger != nil {
		s.logger.Info("tags collected from selected topics", zap.Int("count", len(tagsInTopics)))
	}

	return s.filterTagsDirect(ctx, query, tagsInTopics)
}

func (s *chapterService) filterTopics(ctx context.Context, query string) ([]types.Topic, error) {
	if s.topicRepo == nil {
		return nil, nil
	}
	topics, err := s.topicRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	if len(topics) == 0 {
		return nil, nil
	}

	filterResults, err := s.relCore.FilterTopicsByQuery(ctx, query, topics)
	if err != nil {
		return nil, err
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

func (s *chapterService) getTagsFromTopics(ctx context.Context, topicIDs []string) ([]types.Tag, error) {
	var allTags []types.Tag
	seenTags := make(map[string]bool)

	for _, tid := range topicIDs {
		tags, err := s.tagRepo.ListByTopic(ctx, tid)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to get tags by topic", zap.String("topic_id", tid), zap.Error(err))
			}
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

func extractTagNames(tags []types.Tag) []string {
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = tag.Name
	}
	return names
}

func extractRelevantTags(results []types.TagFilterResult) []string {
	var names []string
	for _, r := range results {
		if r.Relevance >= 0.5 {
			names = append(names, r.Tag)
		}
	}
	return names
}

func (s *chapterService) queryAndFilterDocumentsForHotSearch(ctx context.Context, query string, tags []string, docCap int) ([]types.Document, error) {
	var docs []types.Document
	var err error

	if len(tags) == 0 {
		return nil, nil
	}
	docs, err = s.docRepo.ListByTags(ctx, tags, docCap)
	if err != nil {
		return nil, fmt.Errorf("list documents by tags: %w", err)
	}

	if len(docs) == 0 {
		return nil, nil
	}

	var hotDocs []types.Document
	for _, doc := range docs {
		if doc.Status == types.DocStatusHot || doc.Status == types.DocStatusWarming {
			hotDocs = append(hotDocs, doc)
		}
	}

	if len(hotDocs) == 0 {
		return nil, nil
	}

	filtered, err := s.filterHotDocumentsForHotSearch(ctx, query, hotDocs)
	if err != nil {
		return nil, err
	}
	return filtered, nil
}

func (s *chapterService) filterHotDocumentsForHotSearch(ctx context.Context, query string, docs []types.Document) ([]types.Document, error) {
	filterResults, err := s.relCore.FilterDocuments(ctx, query, docs)
	if err != nil {
		return nil, err
	}
	docMap := make(map[string]types.Document)
	for _, doc := range docs {
		docMap[doc.ID] = doc
	}
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

func (s *chapterService) queryAndFilterChaptersForHotSearch(ctx context.Context, query string, docs []types.Document) ([]types.Chapter, map[string]float64, error) {
	if len(docs) == 0 {
		return nil, nil, nil
	}

	var allChapters []types.Chapter
	for _, doc := range docs {
		chapters, err := s.chapterRepo.ListByDocument(ctx, doc.ID)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to get document chapters", zap.String("doc_id", doc.ID), zap.Error(err))
			}
			continue
		}
		allChapters = append(allChapters, chapters...)
	}

	if len(allChapters) == 0 {
		return nil, nil, nil
	}

	filterResults, err := s.relCore.FilterChapters(ctx, query, allChapters)
	if err != nil {
		return nil, nil, err
	}

	chapterMap := make(map[string]types.Chapter)
	for _, ch := range allChapters {
		chapterMap[ch.Path] = ch
	}

	sort.Slice(filterResults, func(i, j int) bool {
		return filterResults[i].Relevance > filterResults[j].Relevance
	})

	relByPath := make(map[string]float64)
	var filtered []types.Chapter
	for _, fr := range filterResults {
		if fr.Relevance < 0.5 {
			continue
		}
		if ch, ok := chapterMap[fr.ID]; ok {
			filtered = append(filtered, ch)
			relByPath[ch.Path] = fr.Relevance
		}
	}
	return filtered, relByPath, nil
}
