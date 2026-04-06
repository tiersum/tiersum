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

// fieldPattern is used for extracting fields from LLM responses
var fieldPattern = regexp.MustCompile(`(?i)([^:]+):\s*(.+?)(?=\n[^:]*:|\z)`)

// IndexerSvc implements service.IIndexer
type IndexerSvc struct {
	parser     service.IParser
	summarizer service.ISummarizer
	repo       storage.ISummaryRepository
	logger     *zap.Logger
}

// NewIndexerSvc creates a new indexer service
func NewIndexerSvc(parser service.IParser, summarizer service.ISummarizer, repo storage.ISummaryRepository, logger *zap.Logger) *IndexerSvc {
	return &IndexerSvc{
		parser:     parser,
		summarizer: summarizer,
		repo:       repo,
		logger:     logger,
	}
}

// Index implements IIndexer.Index
func (i *IndexerSvc) Index(ctx context.Context, docID string, content string) error {
	parsed, err := i.parser.Parse(content)
	if err != nil {
		return fmt.Errorf("parse document: %w", err)
	}

	summary, err := i.summarizer.Summarize(ctx, content, types.TierDocument)
	if err != nil {
		return fmt.Errorf("summarize document: %w", err)
	}

	sum := &types.Summary{
		DocumentID: docID,
		Tier:       types.TierDocument,
		Path:       "",
		Content:    summary,
	}
	if err := i.repo.Create(ctx, sum); err != nil {
		return fmt.Errorf("create summary: %w", err)
	}

	if err := i.indexNode(ctx, docID, parsed.Root, "", 0); err != nil {
		return err
	}

	return nil
}

func (i *IndexerSvc) indexNode(ctx context.Context, docID string, node *types.ParsedNode, parentPath string, index int) error {
	if node == nil {
		return nil
	}

	path := fmt.Sprintf("%s.%d", parentPath, index)
	if parentPath == "" {
		path = fmt.Sprintf("%d", index)
	}

	if node.Content != "" {
		level := types.TierChapter
		if node.Level > 1 {
			level = types.TierParagraph
		}

		summary, err := i.summarizer.Summarize(ctx, node.Content, level)
		if err != nil {
			i.logger.Error("failed to summarize node", zap.Error(err))
		} else {
			sum := &types.Summary{
				DocumentID: docID,
				Tier:       level,
				Path:       path,
				Content:    summary,
			}
			if err := i.repo.Create(ctx, sum); err != nil {
				return err
			}
		}
	}

	for i, child := range node.Children {
		if err := i.indexNode(ctx, docID, child, path, i); err != nil {
			return err
		}
	}

	return nil
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
func (s *SummarizerSvc) GenerateTopicSummary(ctx context.Context, topicName string, documents []*types.Document) (*types.TopicSummary, error) {
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
	}, nil
}

var _ service.ISummarizer = (*SummarizerSvc)(nil)

// ParserSvc implements service.IParser
type ParserSvc struct{}

// NewParserSvc creates a new parser service
func NewParserSvc() *ParserSvc {
	return &ParserSvc{}
}

// Parse implements IParser.Parse
func (p *ParserSvc) Parse(content string) (*types.ParsedDocument, error) {
	return &types.ParsedDocument{
		Content: content,
		Root:    &types.ParsedNode{Level: 0, Title: "Root"},
	}, nil
}

var _ service.IParser = (*ParserSvc)(nil)

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
