package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/viper"
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
	if len(docs) == 0 {
		return nil, nil
	}
	var docList strings.Builder
	for i, d := range docs {
		docList.WriteString(fmt.Sprintf("[%d] Title: %s\nTags: %v\nSummary: %s\n\n",
			i, d.Title, d.Tags, truncateStringForHotLLM(d.Content, c.config.ParagraphSummaryMax)))
	}
	prompt := fmt.Sprintf(c.filterDocsPrompt, query, docList.String())
	metrics.RecordLLMCall(metrics.PathDocFilter, estimateTokensForHotLLM(prompt))
	resp, err := c.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		c.logger.Error("LLM filter failed", zap.Error(err))
		return c.fallbackFilterDocuments(docs), nil
	}
	return c.parseFilterResults(resp), nil
}

func (c *hotProgressiveLLMCore) FilterChapters(ctx context.Context, query string, chapters []types.Chapter) ([]types.LLMFilterResult, error) {
	if len(chapters) == 0 {
		return nil, nil
	}
	var chapterList strings.Builder
	for i, ch := range chapters {
		body := ch.Summary
		if strings.TrimSpace(body) == "" {
			body = ch.Content
		}
		chapterList.WriteString(fmt.Sprintf("[%d] Path: %s\nSummary: %s\n\n",
			i, ch.Path, truncateStringForHotLLM(body, c.config.ParagraphSummaryMax)))
	}
	prompt := fmt.Sprintf(c.filterChapsPrompt, query, chapterList.String())
	metrics.RecordLLMCall(metrics.PathChapterFilter, estimateTokensForHotLLM(prompt))
	resp, err := c.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		c.logger.Error("LLM chapter filter failed", zap.Error(err))
		return c.fallbackFilterChapters(chapters), nil
	}
	return c.parseFilterResults(resp), nil
}

func (c *hotProgressiveLLMCore) FilterTopicsByQuery(ctx context.Context, query string, topics []types.Topic) ([]types.LLMFilterResult, error) {
	if len(topics) == 0 {
		return nil, nil
	}
	var topicList strings.Builder
	for i, g := range topics {
		topicList.WriteString(fmt.Sprintf("[%d] ID: %s\nName: %s\nDescription: %s\nTag names: %v\n\n",
			i, g.ID, g.Name, g.Description, g.TagNames))
	}
	prompt := fmt.Sprintf(c.filterTopicsPrompt, query, topicList.String())
	metrics.RecordLLMCall(metrics.PathTopicFilter, estimateTokensForHotLLM(prompt))
	resp, err := c.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		c.logger.Error("LLM topic filter failed", zap.Error(err))
		return c.fallbackFilterTopics(topics), nil
	}
	return c.parseFilterResults(resp), nil
}

func (c *hotProgressiveLLMCore) FilterTagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	var tagList strings.Builder
	for _, tag := range tags {
		tagList.WriteString(fmt.Sprintf("- %s (used in %d documents)\n", tag.Name, tag.DocumentCount))
	}
	prompt := fmt.Sprintf(c.filterTagsPrompt, query, tagList.String())
	metrics.RecordLLMCall(metrics.PathTagFilter, estimateTokensForHotLLM(prompt))
	resp, err := c.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		c.logger.Error("LLM tag filter failed", zap.Error(err))
		return c.fallbackTagFilter(tags), nil
	}
	return c.parseTagFilterResults(resp), nil
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

func (c *hotProgressiveLLMCore) fallbackFilterDocuments(docs []types.Document) []types.LLMFilterResult {
	results := make([]types.LLMFilterResult, len(docs))
	for i, doc := range docs {
		results[i] = types.LLMFilterResult{ID: doc.ID, Relevance: 0.5}
	}
	return results
}

func (c *hotProgressiveLLMCore) fallbackFilterChapters(chapters []types.Chapter) []types.LLMFilterResult {
	results := make([]types.LLMFilterResult, len(chapters))
	for i, ch := range chapters {
		results[i] = types.LLMFilterResult{ID: ch.Path, Relevance: 0.5}
	}
	return results
}

func (c *hotProgressiveLLMCore) fallbackFilterTopics(topics []types.Topic) []types.LLMFilterResult {
	results := make([]types.LLMFilterResult, len(topics))
	for i, g := range topics {
		results[i] = types.LLMFilterResult{ID: g.ID, Relevance: 0.5}
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

func (c *hotProgressiveLLMCore) fallbackTagFilter(tags []types.Tag) []types.TagFilterResult {
	results := make([]types.TagFilterResult, len(tags))
	for i, tag := range tags {
		results[i] = types.TagFilterResult{Tag: tag.Name, Relevance: 0.5}
	}
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
