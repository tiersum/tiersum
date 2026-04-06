package svcimpl

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// IndexerSvc implements service.IIndexer
type IndexerSvc struct {
	summarizer service.ISummarizer
	repo       storage.ISummaryRepository
	logger     *zap.Logger
}

// NewIndexerSvc creates a new indexer service
func NewIndexerSvc(summarizer service.ISummarizer, repo storage.ISummaryRepository, logger *zap.Logger) *IndexerSvc {
	return &IndexerSvc{
		summarizer: summarizer,
		repo:       repo,
		logger:     logger,
	}
}

// Index implements IIndexer.Index
// Pre-generates multi-level summaries: Document -> Chapter -> Paragraph
func (i *IndexerSvc) Index(ctx context.Context, docID string, content string) error {
	// L1: Generate Document summary
	docSummary, err := i.summarizer.Summarize(ctx, content, types.TierDocument)
	if err != nil {
		return fmt.Errorf("summarize document: %w", err)
	}

	docSum := &types.Summary{
		DocumentID: docID,
		Tier:       types.TierDocument,
		Path:       "",
		Content:    docSummary,
		IsSource:   false, // Document 层可以深入
	}
	if err := i.repo.Create(ctx, docSum); err != nil {
		return fmt.Errorf("create document summary: %w", err)
	}

	// L2: Parse chapters from markdown
	chapters := i.parseChapters(content)
	for _, chapter := range chapters {
		chapterPath := fmt.Sprintf("%s/%s", docID, sanitizePath(chapter.Title))

		// Check if chapter is short (< 1000 chars)
		if len(chapter.Content) < 1000 {
			// Short chapter: store as source directly, no paragraph summaries
			shortChapterSum := &types.Summary{
				DocumentID: docID,
				Tier:       types.TierChapter,
				Path:       chapterPath,
				Content:    chapter.Content, // Store original content
				IsSource:   true,           // Mark as source (cannot drill down)
			}
			if err := i.repo.Create(ctx, shortChapterSum); err != nil {
				return fmt.Errorf("create short chapter summary: %w", err)
			}
			i.logger.Debug("stored short chapter as source",
				zap.String("path", chapterPath),
				zap.Int("length", len(chapter.Content)))
			continue
		}

		// Long chapter: generate summary and paragraph summaries
		chapterSummary, err := i.summarizer.Summarize(ctx, chapter.Content, types.TierChapter)
		if err != nil {
			i.logger.Warn("failed to summarize chapter", zap.String("path", chapterPath), zap.Error(err))
			continue
		}

		chapterSum := &types.Summary{
			DocumentID: docID,
			Tier:       types.TierChapter,
			Path:       chapterPath,
			Content:    chapterSummary,
			IsSource:   false, // Long chapter can drill down to paragraphs
		}
		if err := i.repo.Create(ctx, chapterSum); err != nil {
			return fmt.Errorf("create chapter summary: %w", err)
		}

		// L3: Split into paragraphs and generate summaries
		paragraphs := i.splitParagraphsByLLM(ctx, chapter.Content)
		for pIdx, para := range paragraphs {
			paraPath := fmt.Sprintf("%s/%d", chapterPath, pIdx)

			paraSummary, err := i.summarizer.Summarize(ctx, para.Content, types.TierParagraph)
			if err != nil {
				i.logger.Warn("failed to summarize paragraph", zap.String("path", paraPath), zap.Error(err))
				continue
			}

			paraSum := &types.Summary{
				DocumentID: docID,
				Tier:       types.TierParagraph,
				Path:       paraPath,
				Content:    paraSummary,
				IsSource:   false, // Paragraph can drill down to source
			}
			if err := i.repo.Create(ctx, paraSum); err != nil {
				return fmt.Errorf("create paragraph summary: %w", err)
			}
		}

		i.logger.Debug("indexed chapter with paragraphs",
			zap.String("path", chapterPath),
			zap.Int("paragraphs", len(paragraphs)))
	}

	i.logger.Info("completed multi-level indexing",
		zap.String("doc_id", docID),
		zap.Int("chapters", len(chapters)))

	return nil
}

// parseChapters parses markdown content into chapters based on headings
func (i *IndexerSvc) parseChapters(content string) []types.ChapterInfo {
	var chapters []types.ChapterInfo

	// Match markdown headings: ## Heading or ### Heading
	// Group 1: heading level (number of #)
	// Group 2: heading text
	headingRegex := regexp.MustCompile(`(?m)^(#{2,3})\s+(.+)$`)
	matches := headingRegex.FindAllStringIndex(content, -1)

	if len(matches) == 0 {
		// No chapters found, treat entire content as single chapter
		return []types.ChapterInfo{{
			Title:   "Content",
			Level:   2,
			Content: strings.TrimSpace(content),
			Offset:  0,
		}}
	}

	for i, match := range matches {
		// Get heading line
		headingLine := content[match[0]:match[1]]
		parts := headingRegex.FindStringSubmatch(headingLine)
		if len(parts) < 3 {
			continue
		}

		level := len(parts[1]) // Number of #
		title := strings.TrimSpace(parts[2])

		// Calculate content range
		contentStart := match[1]
		contentEnd := len(content)
		if i < len(matches)-1 {
			contentEnd = matches[i+1][0]
		}

		chapterContent := strings.TrimSpace(content[contentStart:contentEnd])

		chapters = append(chapters, types.ChapterInfo{
			Title:   title,
			Level:   level,
			Content: chapterContent,
			Offset:  match[0],
		})
	}

	return chapters
}

// splitParagraphsByLLM uses LLM to intelligently split content into semantic paragraphs
func (i *IndexerSvc) splitParagraphsByLLM(ctx context.Context, content string) []types.ParagraphInfo {
	// For now, use simple splitting by blank lines
	// TODO: Use LLM for semantic segmentation when needed

	// Split by double newline (paragraph separator)
	parts := regexp.MustCompile(`\n\s*\n`).Split(content, -1)

	var paragraphs []types.ParagraphInfo
	offset := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			offset += len(part) + 2 // Account for separator
			continue
		}

		paragraphs = append(paragraphs, types.ParagraphInfo{
			Content: part,
			Offset:  offset,
		})

		offset += len(part) + 2
	}

	// If no paragraphs found or only one, try splitting by sentences
	if len(paragraphs) <= 1 && len(content) > 500 {
		return i.splitBySentences(content)
	}

	return paragraphs
}

// splitBySentences splits content by sentences for very long single paragraphs
func (i *IndexerSvc) splitBySentences(content string) []types.ParagraphInfo {
	// Simple sentence splitting (period followed by space or newline)
	sentenceRegex := regexp.MustCompile(`[.!?。！？]\s+`)
	sentences := sentenceRegex.Split(content, -1)

	var paragraphs []types.ParagraphInfo
	var currentPara strings.Builder
	offset := 0
	paraStart := 0

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		// Add period back
		sentence += "."

		if currentPara.Len()+len(sentence) > 400 {
			// Start new paragraph
			if currentPara.Len() > 0 {
				paragraphs = append(paragraphs, types.ParagraphInfo{
					Content: strings.TrimSpace(currentPara.String()),
					Offset:  paraStart,
				})
			}
			currentPara.Reset()
			currentPara.WriteString(sentence)
			paraStart = offset
		} else {
			if currentPara.Len() > 0 {
				currentPara.WriteString(" ")
			}
			currentPara.WriteString(sentence)
		}

		offset += len(sentence) + 1
	}

	// Add remaining content
	if currentPara.Len() > 0 {
		paragraphs = append(paragraphs, types.ParagraphInfo{
			Content: strings.TrimSpace(currentPara.String()),
			Offset:  paraStart,
		})
	}

	return paragraphs
}

// sanitizePath sanitizes a string for use in path
func sanitizePath(s string) string {
	// Replace slashes and other special chars
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.TrimSpace(s)
	// Limit length
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

var _ service.IIndexer = (*IndexerSvc)(nil)

// SummarizerSvc implements service.ISummarizer
type SummarizerSvc struct {
	provider client.ILLMProvider
	logger   *zap.Logger
	config   SummarizerConfig
}

// SummarizerConfig holds summarizer configuration
type SummarizerConfig struct {
	DocumentMaxLength  int
	ChapterMaxLength   int
	ParagraphMaxLength int
}

// NewSummarizerSvc creates a new summarizer service
func NewSummarizerSvc(provider client.ILLMProvider, logger *zap.Logger) *SummarizerSvc {
	config := SummarizerConfig{
		DocumentMaxLength:  viper.GetInt("summarization.document_summary_max"),
		ChapterMaxLength:   viper.GetInt("summarization.chapter_summary_max"),
		ParagraphMaxLength: viper.GetInt("summarization.paragraph_summary_max"),
	}
	if config.DocumentMaxLength == 0 {
		config.DocumentMaxLength = 500
	}
	if config.ChapterMaxLength == 0 {
		config.ChapterMaxLength = 300
	}
	if config.ParagraphMaxLength == 0 {
		config.ParagraphMaxLength = 150
	}

	return &SummarizerSvc{
		provider: provider,
		logger:   logger,
		config:   config,
	}
}

// Summarize implements ISummarizer.Summarize
func (s *SummarizerSvc) Summarize(ctx context.Context, content string, level types.SummaryTier) (string, error) {
	maxLength := s.config.DocumentMaxLength
	switch level {
	case types.TierChapter:
		maxLength = s.config.ChapterMaxLength
	case types.TierParagraph:
		maxLength = s.config.ParagraphMaxLength
	}

	prompt := fmt.Sprintf("Summarize the following content in %d characters or less:\n\n%s", maxLength, content)
	return s.provider.Generate(ctx, prompt, maxLength)
}

// AnalyzeDocument implements ISummarizer.AnalyzeDocument
func (s *SummarizerSvc) AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	maxTokens := 1000

	prompt := fmt.Sprintf(`Analyze the following document and provide a structured response.

Title: %s

Content:
%s

Please provide your analysis in the following format:

SUMMARY: A concise summary of the document (max 300 characters)

TAGS: 3-5 relevant tags separated by commas

TOPIC: The main topic (1-3 words)

KEY_POINTS: 3-5 bullet points

Your response:`, title, truncateContent(content, 8000))

	response, err := s.provider.Generate(ctx, prompt, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	return parseAnalysisResponse(response), nil
}

// GenerateTopicSummary implements ISummarizer.GenerateTopicSummary
func (s *SummarizerSvc) GenerateTopicSummary(ctx context.Context, topicName string, documents []*types.Document, source types.TopicSource) (*types.TopicSummary, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents provided")
	}

	var docContext string
	for i, doc := range documents {
		docContext += fmt.Sprintf("\n--- Document %d ---\nTitle: %s\nTags: %v\nContent: %s\n",
			i+1, doc.Title, doc.Tags, truncateContent(doc.Content, 2000))
	}

	prompt := fmt.Sprintf(`Create a topic summary for "%s" based on these documents:
%s

Format:
DESCRIPTION: Brief description (max 200 chars)
SUMMARY: Comprehensive summary (max 500 chars)
TAGS: 5-8 tags separated by commas

Your response:`, topicName, docContext)

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	description := extractField(response, "DESCRIPTION")
	summary := extractField(response, "SUMMARY")
	tags := parseTags(extractField(response, "TAGS"))

	docIDs := make([]string, len(documents))
	for i, doc := range documents {
		docIDs[i] = doc.ID
	}

	return &types.TopicSummary{
		Name:        topicName,
		Description: description,
		Summary:     summary,
		Tags:        tags,
		DocumentIDs: docIDs,
		Source:      source,
	}, nil
}

var _ service.ISummarizer = (*SummarizerSvc)(nil)

// Helper functions
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

func parseAnalysisResponse(response string) *types.DocumentAnalysisResult {
	result := &types.DocumentAnalysisResult{
		Tags:      []string{},
		KeyPoints: []string{},
	}

	result.Summary = extractField(response, "SUMMARY")
	result.Topic = extractField(response, "TOPIC")
	result.Tags = parseTags(extractField(response, "TAGS"))

	keyPointsSection := extractField(response, "KEY_POINTS")
	if keyPointsSection != "" {
		lines := splitLines(keyPointsSection)
		for _, line := range lines {
			line = cleanLine(line)
			if line != "" {
				result.KeyPoints = append(result.KeyPoints, line)
			}
		}
	}

	return result
}

func extractField(response, fieldName string) string {
	// Use pre-compiled pattern and replace field name placeholder
	patternStr := fmt.Sprintf(`(?i)%s[:：]\s*(.+?)(?=\n\w+[:：]|\z)`, regexp.QuoteMeta(fieldName))
	pattern := regexp.MustCompile(patternStr)
	matches := pattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func parseTags(tagStr string) []string {
	if tagStr == "" {
		return []string{}
	}
	parts := strings.Split(tagStr, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		tag = strings.TrimPrefix(tag, "-")
		tag = strings.TrimPrefix(tag, "•")
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, strings.ToLower(tag))
		}
	}
	return tags
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func cleanLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "-")
	line = strings.TrimPrefix(line, "•")
	return strings.TrimSpace(line)
}
