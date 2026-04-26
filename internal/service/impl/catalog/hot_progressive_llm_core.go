package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/pkg/metrics"
	"github.com/tiersum/tiersum/pkg/types"
)

type hotProgressiveLLMConfig struct {
	ParagraphSummaryMax int
}

// hotProgressiveLLMCore holds LLM prompts for progressive hot search (tags, topics, documents, chapters).
type hotProgressiveLLMCore struct {
	provider           client.ILLMProvider
	logger             *zap.Logger
	config             hotProgressiveLLMConfig
	filterDocsPrompt   string
	filterChapsPrompt  string
	filterTopicsPrompt string
	filterTagsPrompt   string
}

func newHotProgressiveLLMCore(provider client.ILLMProvider, logger *zap.Logger, filterDocs, filterChaps, filterTopics, filterTags string) *hotProgressiveLLMCore {
	cfg := hotProgressiveLLMConfig{
		ParagraphSummaryMax: viper.GetInt("summarization.paragraph_summary_max"),
	}
	if cfg.ParagraphSummaryMax == 0 {
		cfg.ParagraphSummaryMax = 300
	}
	return &hotProgressiveLLMCore{
		provider:           provider,
		logger:             logger,
		config:             cfg,
		filterDocsPrompt:   filterDocs,
		filterChapsPrompt:  filterChaps,
		filterTopicsPrompt: filterTopics,
		filterTagsPrompt:   filterTags,
	}
}

func (c *hotProgressiveLLMCore) FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "FilterDocuments", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("query", query))
	span.SetAttributes(attribute.Int("input_doc_count", len(docs)))
	if len(docs) > 0 {
		ids := make([]string, 0, min(10, len(docs)))
		for i := 0; i < min(10, len(docs)); i++ {
			ids = append(ids, docs[i].ID)
		}
		span.SetAttributes(attribute.String("input_doc_ids", joinFirstN(ids, 10)))
	}

	if len(docs) == 0 {
		return nil, nil
	}
	var docList strings.Builder
	for _, d := range docs {
		summary := strings.TrimSpace(d.Summary)
		if summary == "" {
			continue
		}
		docList.WriteString(fmt.Sprintf("ID: %s\nTitle: %s\nTags: %v\nSummary: %s\n\n",
			d.ID, d.Title, d.Tags, summary))
	}
	dataContent := fmt.Sprintf("Query: %s\n\nDocuments:\n%s", query, docList.String())
	msgs := []client.LLMMessage{
		{Role: client.LLMMessageRoleSystem, Content: c.filterDocsPrompt},
		{Role: client.LLMMessageRoleUser, Content: dataContent},
	}
	resp, err := c.provider.Generate(ctx, msgs, 1500)
	if err != nil {
		c.logger.Error("LLM filter failed", zap.Error(err))
		return nil, err
	}
	metrics.RecordLLMTokens(metrics.PathDocFilter, estimateTokensForHotLLM(dataContent), estimateTokensForHotLLM(resp))
	results := c.parseFilterResults(resp)
	if len(results) > 0 {
		ids := make([]string, 0, min(10, len(results)))
		for i := 0; i < min(10, len(results)); i++ {
			ids = append(ids, results[i].ID)
		}
		span.SetAttributes(attribute.Int("output_result_count", len(results)))
		span.SetAttributes(attribute.String("output_result_ids", joinFirstN(ids, 10)))
	}
	return results, nil
}

func (c *hotProgressiveLLMCore) FilterChapters(ctx context.Context, query string, chapters []types.Chapter) ([]types.LLMFilterResult, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "FilterChapters", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("query", query))
	span.SetAttributes(attribute.Int("input_chapter_count", len(chapters)))
	if len(chapters) > 0 {
		paths := make([]string, 0, min(10, len(chapters)))
		for i := 0; i < min(10, len(chapters)); i++ {
			paths = append(paths, chapters[i].Path)
		}
		span.SetAttributes(attribute.String("input_paths", joinFirstN(paths, 10)))
	}

	if len(chapters) == 0 {
		return nil, nil
	}
	var chapterList strings.Builder
	for _, ch := range chapters {
		body := strings.TrimSpace(ch.Summary)
		if body == "" {
			continue
		}
		chapterList.WriteString(fmt.Sprintf("Path: %s\nSummary: %s\n\n",
			ch.Path, truncateStringForHotLLM(body, c.config.ParagraphSummaryMax)))
	}
	if chapterList.Len() == 0 {
		return nil, nil
	}
	dataContent := fmt.Sprintf("Query: %s\n\nChapters:\n%s", query, chapterList.String())
	msgs := []client.LLMMessage{
		{Role: client.LLMMessageRoleSystem, Content: c.filterChapsPrompt},
		{Role: client.LLMMessageRoleUser, Content: dataContent},
	}
	resp, err := c.provider.Generate(ctx, msgs, 1500)
	if err != nil {
		c.logger.Error("LLM chapter filter failed", zap.Error(err))
		return nil, err
	}
	metrics.RecordLLMTokens(metrics.PathChapterFilter, estimateTokensForHotLLM(dataContent), estimateTokensForHotLLM(resp))
	results := c.parseFilterResults(resp)
	if len(results) > 0 {
		ids := make([]string, 0, min(10, len(results)))
		for i := 0; i < min(10, len(results)); i++ {
			ids = append(ids, results[i].ID)
		}
		span.SetAttributes(attribute.Int("output_result_count", len(results)))
		span.SetAttributes(attribute.String("output_result_ids", joinFirstN(ids, 10)))
	}
	return results, nil
}

func (c *hotProgressiveLLMCore) FilterTopicsByQuery(ctx context.Context, query string, topics []types.Topic) ([]types.LLMFilterResult, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "FilterTopicsByQuery", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("query", query))
	span.SetAttributes(attribute.Int("input_topic_count", len(topics)))
	if len(topics) > 0 {
		ids := make([]string, 0, min(10, len(topics)))
		for i := 0; i < min(10, len(topics)); i++ {
			ids = append(ids, topics[i].ID)
		}
		span.SetAttributes(attribute.String("input_topic_ids", joinFirstN(ids, 10)))
	}

	if len(topics) == 0 {
		return nil, nil
	}
	var topicList strings.Builder
	for i, g := range topics {
		topicList.WriteString(fmt.Sprintf("[%d] ID: %s\nName: %s\nDescription: %s\nTag names: %v\n\n",
			i, g.ID, g.Name, g.Description, g.TagNames))
	}
	dataContent := fmt.Sprintf("Query: %s\n\nAvailable topics:\n%s", query, topicList.String())
	msgs := []client.LLMMessage{
		{Role: client.LLMMessageRoleSystem, Content: c.filterTopicsPrompt},
		{Role: client.LLMMessageRoleUser, Content: dataContent},
	}
	resp, err := c.provider.Generate(ctx, msgs, 1500)
	if err != nil {
		c.logger.Error("LLM topic filter failed", zap.Error(err))
		return nil, err
	}
	metrics.RecordLLMTokens(metrics.PathTopicFilter, estimateTokensForHotLLM(dataContent), estimateTokensForHotLLM(resp))
	results := c.parseFilterResults(resp)
	if len(results) > 0 {
		ids := make([]string, 0, min(10, len(results)))
		for i := 0; i < min(10, len(results)); i++ {
			ids = append(ids, results[i].ID)
		}
		span.SetAttributes(attribute.Int("output_result_count", len(results)))
		span.SetAttributes(attribute.String("output_result_ids", joinFirstN(ids, 10)))
	}
	return results, nil
}

func (c *hotProgressiveLLMCore) FilterTagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/catalog")
	ctx, span := tr.Start(ctx, "FilterTagsByQuery", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("query", query))
	span.SetAttributes(attribute.Int("input_tag_count", len(tags)))
	if len(tags) > 0 {
		names := make([]string, 0, min(10, len(tags)))
		for i := 0; i < min(10, len(tags)); i++ {
			names = append(names, tags[i].Name)
		}
		span.SetAttributes(attribute.String("input_tags", joinFirstN(names, 10)))
	}

	if len(tags) == 0 {
		return nil, nil
	}
	var tagList strings.Builder
	for _, tag := range tags {
		tagList.WriteString(fmt.Sprintf("- %s (used in %d documents)\n", tag.Name, tag.DocumentCount))
	}
	dataContent := fmt.Sprintf("Query: %s\n\nAvailable tags:\n%s", query, tagList.String())
	msgs := []client.LLMMessage{
		{Role: client.LLMMessageRoleSystem, Content: c.filterTagsPrompt},
		{Role: client.LLMMessageRoleUser, Content: dataContent},
	}
	resp, err := c.provider.Generate(ctx, msgs, 1500)
	if err != nil {
		c.logger.Error("LLM tag filter failed", zap.Error(err))
		return nil, err
	}
	metrics.RecordLLMTokens(metrics.PathTagFilter, estimateTokensForHotLLM(dataContent), estimateTokensForHotLLM(resp))
	results := c.parseTagFilterResults(resp)
	if len(results) > 0 {
		names := make([]string, 0, min(10, len(results)))
		for i := 0; i < min(10, len(results)); i++ {
			names = append(names, results[i].Tag)
		}
		span.SetAttributes(attribute.Int("output_result_count", len(results)))
		span.SetAttributes(attribute.String("output_tags", joinFirstN(names, 10)))
	}
	return results, nil
}

func (c *hotProgressiveLLMCore) parseFilterResults(response string) []types.LLMFilterResult {
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil
	}
	jsonStr := response[jsonStart : jsonEnd+1]
	var results []types.LLMFilterResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		c.logger.Warn("failed to parse filter results", zap.Error(err))
		return nil
	}
	return results
}

func (c *hotProgressiveLLMCore) parseTagFilterResults(response string) []types.TagFilterResult {
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil
	}
	jsonStr := response[jsonStart : jsonEnd+1]
	var results []types.TagFilterResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		c.logger.Warn("failed to parse tag filter results", zap.Error(err))
		return nil
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})
	return results
}

func truncateStringForHotLLM(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func estimateTokensForHotLLM(text string) int {
	if text == "" {
		return 0
	}
	charCount := len(text)
	chineseCount := 0
	for _, r := range text {
		if r > 127 {
			chineseCount++
		}
	}
	if chineseCount > 0 {
		englishChars := charCount - chineseCount
		return chineseCount + englishChars/4
	}
	return charCount / 4
}

func joinFirstN(items []string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	return strings.Join(items, ",")
}
