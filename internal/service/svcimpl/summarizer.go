package svcimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// SummarizerSvc implements service.ISummarizer
type SummarizerSvc struct {
	provider client.ILLMProvider
	logger   *zap.Logger
	config   SummarizerConfig
}

// SummarizerConfig holds summarizer configuration
type SummarizerConfig struct {
	DocumentSummaryMax  int
	ChapterSummaryMax   int
	MaxTagsPerDocument  int
}

// NewSummarizerSvc creates a new summarizer service
func NewSummarizerSvc(provider client.ILLMProvider, logger *zap.Logger) *SummarizerSvc {
	config := SummarizerConfig{
		DocumentSummaryMax:  viper.GetInt("summarization.document_summary_max"),
		ChapterSummaryMax:   viper.GetInt("summarization.chapter_summary_max"),
		MaxTagsPerDocument:  10,
	}
	if config.DocumentSummaryMax == 0 {
		config.DocumentSummaryMax = 300
	}
	if config.ChapterSummaryMax == 0 {
		config.ChapterSummaryMax = 200
	}

	return &SummarizerSvc{
		provider: provider,
		logger:   logger,
		config:   config,
	}
}

// AnalyzeDocument implements ISummarizer.AnalyzeDocument
// Uses a single prompt to generate: document summary, tags (max 10), and chapter summaries with content
func (s *SummarizerSvc) AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	// First, extract chapters from the document
	chapters := s.extractChapters(content)

	// Build chapter context for the prompt
	var chapterContext strings.Builder
	for i, ch := range chapters {
		chapterContext.WriteString(fmt.Sprintf("\nChapter %d: %s\n", i+1, ch.Title))
		chapterContext.WriteString(fmt.Sprintf("Content preview: %s\n", truncateString(ch.Content, 500)))
	}

	prompt := fmt.Sprintf(`Analyze the following document and provide a JSON response.

Title: %s

Full Content:
%s

Chapters identified:%s

Please analyze this document and return a JSON object with the following structure:
{
  "summary": "document summary (max %d chars)",
  "tags": ["tag1", "tag2", ...], // Up to %d tags
  "chapters": [
    {
      "title": "chapter title",
      "summary": "chapter summary (max %d chars)",
      "content": "full chapter content"
    },
    ...
  ]
}

Guidelines:
- Summary should capture the main points of the entire document
- Tags should be relevant keywords (lowercase, no spaces use-hyphens)
- Each chapter should have a clear title, concise summary, and full content
- If the document has no clear chapters, create a single chapter with the full content

Return ONLY the JSON object, no other text.`,
		title,
		truncateString(content, 10000),
		chapterContext.String(),
		s.config.DocumentSummaryMax,
		s.config.MaxTagsPerDocument,
		s.config.ChapterSummaryMax,
	)

	response, err := s.provider.Generate(ctx, prompt, 4000)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	result, err := s.parseAnalysisResponse(response)
	if err != nil {
		s.logger.Warn("failed to parse LLM response, using fallback", zap.Error(err))
		return s.fallbackAnalysis(title, content, chapters), nil
	}

	// Validate and limit tags
	if len(result.Tags) > s.config.MaxTagsPerDocument {
		result.Tags = result.Tags[:s.config.MaxTagsPerDocument]
	}

	// Ensure each tag is lowercase and trimmed
	for i, tag := range result.Tags {
		result.Tags[i] = strings.ToLower(strings.TrimSpace(tag))
	}

	return result, nil
}

// FilterDocuments implements ISummarizer.FilterDocuments
func (s *SummarizerSvc) FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Build prompt with document info
	var docList strings.Builder
	for i, d := range docs {
		docList.WriteString(fmt.Sprintf("[%d] Title: %s\nTags: %v\nSummary: %s\n\n",
			i, d.Title, d.Tags, truncateString(d.Content, 300)))
	}

	prompt := fmt.Sprintf(`Given the query: "%s"

Evaluate the relevance of each document below. Return a JSON array of objects with fields "id" (document ID) and "relevance" (0.0-1.0 score).
Only include documents with relevance >= 0.5. Sort by relevance descending.

Documents:
%s

Response format (JSON only):
[
  {"id": "doc_id", "relevance": 0.95},
  ...
]`, query, docList.String())

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		s.logger.Error("LLM filter failed", zap.Error(err))
		return s.fallbackFilterDocuments(docs), nil
	}

	return s.parseFilterResults(response), nil
}

// FilterChapters implements ISummarizer.FilterChapters
func (s *SummarizerSvc) FilterChapters(ctx context.Context, query string, chapters []types.Summary) ([]types.LLMFilterResult, error) {
	if len(chapters) == 0 {
		return nil, nil
	}

	// Build prompt
	var chapterList strings.Builder
	for i, ch := range chapters {
		chapterList.WriteString(fmt.Sprintf("[%d] Path: %s\nSummary: %s\n\n",
			i, ch.Path, truncateString(ch.Content, 300)))
	}

	prompt := fmt.Sprintf(`Given the query: "%s"

Evaluate the relevance of each chapter below. Return a JSON array of objects with fields "id" (the path) and "relevance" (0.0-1.0 score).
Only include chapters with relevance >= 0.5. Sort by relevance descending.

Chapters:
%s

Response format (JSON only):
[
  {"id": "doc_id/chapter_title", "relevance": 0.88},
  ...
]`, query, chapterList.String())

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		s.logger.Error("LLM chapter filter failed", zap.Error(err))
		return s.fallbackFilterChapters(chapters), nil
	}

	return s.parseFilterResults(response), nil
}

// extractChapters extracts chapters from markdown content
func (s *SummarizerSvc) extractChapters(content string) []types.ChapterInfo {
	var chapters []types.ChapterInfo

	// Match markdown headings: ## Heading or ### Heading
	headingRegex := regexp.MustCompile(`(?m)^(#{1,3})\s+(.+)$`)
	matches := headingRegex.FindAllStringIndex(content, -1)

	if len(matches) == 0 {
		// No chapters found, treat entire content as single chapter
		return []types.ChapterInfo{{
			Title:   "Content",
			Level:   1,
			Content: strings.TrimSpace(content),
		}}
	}

	for i, match := range matches {
		headingLine := content[match[0]:match[1]]
		parts := headingRegex.FindStringSubmatch(headingLine)
		if len(parts) < 3 {
			continue
		}

		level := len(parts[1])
		title := strings.TrimSpace(parts[2])

		contentStart := match[1]
		contentEnd := len(content)
		if i < len(matches)-1 {
			contentEnd = matches[i+1][0]
		}

		chapters = append(chapters, types.ChapterInfo{
			Title:   title,
			Level:   level,
			Content: strings.TrimSpace(content[contentStart:contentEnd]),
		})
	}

	return chapters
}

// parseAnalysisResponse parses the LLM JSON response
func (s *SummarizerSvc) parseAnalysisResponse(response string) (*types.DocumentAnalysisResult, error) {
	// Extract JSON from response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var result struct {
		Summary  string `json:"summary"`
		Tags     []string `json:"tags"`
		Chapters []struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Content string `json:"content"`
		} `json:"chapters"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	chapters := make([]types.ChapterInfo, len(result.Chapters))
	for i, ch := range result.Chapters {
		chapters[i] = types.ChapterInfo{
			Title:   ch.Title,
			Summary: ch.Summary,
			Content: ch.Content,
		}
	}

	return &types.DocumentAnalysisResult{
		Summary:  result.Summary,
		Tags:     result.Tags,
		Chapters: chapters,
	}, nil
}

// parseFilterResults parses LLM JSON response to filter results
func (s *SummarizerSvc) parseFilterResults(response string) []types.LLMFilterResult {
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var results []types.LLMFilterResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		s.logger.Warn("failed to parse filter results", zap.Error(err))
		return nil
	}

	return results
}

// fallbackAnalysis provides a basic analysis when LLM fails
func (s *SummarizerSvc) fallbackAnalysis(title string, content string, chapters []types.ChapterInfo) *types.DocumentAnalysisResult {
	// Simple summary: first 200 chars
	summary := truncateString(content, 200)
	if len(content) > 200 {
		summary += "..."
	}

	// Simple tags from title words
	words := strings.Fields(strings.ToLower(title))
	tags := make([]string, 0, 5)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 && len(tags) < 5 {
			tags = append(tags, word)
		}
	}

	return &types.DocumentAnalysisResult{
		Summary:  summary,
		Tags:     tags,
		Chapters: chapters,
	}
}

// fallbackFilterDocuments returns all documents with equal relevance
func (s *SummarizerSvc) fallbackFilterDocuments(docs []types.Document) []types.LLMFilterResult {
	results := make([]types.LLMFilterResult, len(docs))
	for i, doc := range docs {
		results[i] = types.LLMFilterResult{
			ID:        doc.ID,
			Relevance: 0.5,
		}
	}
	return results
}

// fallbackFilterChapters returns all chapters with equal relevance
func (s *SummarizerSvc) fallbackFilterChapters(chapters []types.Summary) []types.LLMFilterResult {
	results := make([]types.LLMFilterResult, len(chapters))
	for i, ch := range chapters {
		results[i] = types.LLMFilterResult{
			ID:        ch.Path,
			Relevance: 0.5,
		}
	}
	return results
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var _ service.ISummarizer = (*SummarizerSvc)(nil)
var _ service.ILLMFilter = (*SummarizerSvc)(nil)
