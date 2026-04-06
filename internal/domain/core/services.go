// Package core implements domain logic interfaces defined in ports
package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/pkg/types"
)

// ParserSvc implements ports.Parser
type ParserSvc struct {
	// Add parser dependencies here
}

// NewParserSvc creates a new parser service
func NewParserSvc() *ParserSvc {
	return &ParserSvc{}
}

// Parse implements ports.Parser.Parse
func (p *ParserSvc) Parse(content string) (*types.ParsedDocument, error) {
	// TODO: Implement actual parsing logic
	return &types.ParsedDocument{
		Content: content,
		Root:    &types.ParsedNode{Level: 0, Title: "Root"},
	}, nil
}

// SummarizerSvc implements ports.Summarizer
type SummarizerSvc struct {
	provider ports.LLMProvider
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
func NewSummarizerSvc(provider ports.LLMProvider, logger *zap.Logger) *SummarizerSvc {
	config := SummarizerConfig{
		DocumentMaxLength:  viper.GetInt("summarization.document_summary_max"),
		ChapterMaxLength:   viper.GetInt("summarization.chapter_summary_max"),
		ParagraphMaxLength: viper.GetInt("summarization.paragraph_summary_max"),
	}
	// Set defaults
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

// Summarize implements ports.Summarizer.Summarize
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

// AnalyzeDocument implements ports.Summarizer.AnalyzeDocument
// Uses LLM to generate summary, tags, topic, and key points for a document
func (s *SummarizerSvc) AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error) {
	maxTokens := 1000

	prompt := fmt.Sprintf(`Analyze the following document and provide a structured response.

Title: %s

Content:
%s

Please provide your analysis in the following format:

SUMMARY: A concise summary of the document (max 300 characters)

TAGS: 3-5 relevant tags separated by commas (e.g., technology, ai, tutorial)

TOPIC: The main topic or theme of this document (1-3 words)

KEY_POINTS: 3-5 bullet points of the main takeaways

Your response:`, title, truncateContent(content, 8000))

	response, err := s.provider.Generate(ctx, prompt, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	return parseAnalysisResponse(response), nil
}

// GenerateTopicSummary implements ports.Summarizer.GenerateTopicSummary
// Creates a topic-level summary from multiple documents
func (s *SummarizerSvc) GenerateTopicSummary(ctx context.Context, topicName string, documents []*types.Document) (*types.TopicSummary, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents provided for topic summary")
	}

	// Build context from all documents
	var docContext string
	for i, doc := range documents {
		docContext += fmt.Sprintf("\n--- Document %d ---\nTitle: %s\nTags: %v\nContent Preview: %s\n",
			i+1, doc.Title, doc.Tags, truncateContent(doc.Content, 2000))
	}

	prompt := fmt.Sprintf(`Create a comprehensive topic summary based on the following documents related to "%s".

Documents:%s

Please provide your analysis in the following format:

DESCRIPTION: A brief description of this topic (max 200 characters)

SUMMARY: A comprehensive summary synthesizing all documents (max 500 characters)

TAGS: 5-8 relevant tags for this topic separated by commas

Your response:`, topicName, docContext)

	response, err := s.provider.Generate(ctx, prompt, 1500)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse response
	description := extractField(response, "DESCRIPTION")
	summary := extractField(response, "SUMMARY")
	tags := parseTags(extractField(response, "TAGS"))

	// Collect document IDs
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

// truncateContent truncates content to max length with ellipsis
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// parseAnalysisResponse parses the LLM response into DocumentAnalysisResult
func parseAnalysisResponse(response string) *types.DocumentAnalysisResult {
	result := &types.DocumentAnalysisResult{
		Tags:      []string{},
		KeyPoints: []string{},
	}

	result.Summary = extractField(response, "SUMMARY")
	result.Topic = extractField(response, "TOPIC")
	result.Tags = parseTags(extractField(response, "TAGS"))

	// Parse key points
	keyPointsSection := extractField(response, "KEY_POINTS")
	if keyPointsSection != "" {
		lines := strings.Split(keyPointsSection, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Remove bullet points and dashes
			line = strings.TrimPrefix(line, "-")
			line = strings.TrimPrefix(line, "•")
			line = strings.TrimSpace(line)
			if line != "" {
				result.KeyPoints = append(result.KeyPoints, line)
			}
		}
	}

	return result
}

// extractField extracts a field value from the LLM response
func extractField(response, fieldName string) string {
	// Look for field pattern like "FIELD_NAME: value"
	pattern := regexp.MustCompile(fmt.Sprintf(`(?i)%s[:：]\s*(.+?)(?=\n\w+[:：]|\z)`, regexp.QuoteMeta(fieldName)))
	matches := pattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// parseTags parses comma-separated tags
func parseTags(tagStr string) []string {
	if tagStr == "" {
		return []string{}
	}

	parts := strings.Split(tagStr, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		// Remove common separators and formatting
		tag = strings.TrimPrefix(tag, "-")
		tag = strings.TrimPrefix(tag, "•")
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, strings.ToLower(tag))
		}
	}
	return tags
}

// IndexerSvc implements ports.Indexer
type IndexerSvc struct {
	parser     ports.Parser
	summarizer ports.Summarizer
	repo       ports.SummaryRepository
	logger     *zap.Logger
}

// NewIndexerSvc creates a new indexer service
func NewIndexerSvc(parser ports.Parser, summarizer ports.Summarizer, repo ports.SummaryRepository, logger *zap.Logger) *IndexerSvc {
	return &IndexerSvc{
		parser:     parser,
		summarizer: summarizer,
		repo:       repo,
		logger:     logger,
	}
}

// Index implements ports.Indexer.Index
func (i *IndexerSvc) Index(ctx context.Context, docID string, content string) error {
	// Parse document
	parsed, err := i.parser.Parse(content)
	if err != nil {
		return fmt.Errorf("parse document: %w", err)
	}

	// Generate document-level summary
	summary, err := i.summarizer.Summarize(ctx, content, types.TierDocument)
	if err != nil {
		return fmt.Errorf("summarize document: %w", err)
	}

	// Store document summary
	sum := &types.Summary{
		DocumentID: docID,
		Tier:       types.TierDocument,
		Path:       "",
		Content:    summary,
	}
	if err := i.repo.Create(ctx, sum); err != nil {
		return fmt.Errorf("create summary: %w", err)
	}

	// Process nodes recursively
	if err := i.indexNode(ctx, docID, parsed.Root, "", 0); err != nil {
		return err
	}

	return nil
}

func (i *IndexerSvc) indexNode(ctx context.Context, docID string, node *types.ParsedNode, parentPath string, index int) error {
	if node == nil {
		return nil
	}

	// Build path
	path := fmt.Sprintf("%s.%d", parentPath, index)
	if parentPath == "" {
		path = fmt.Sprintf("%d", index)
	}

	// Generate summary for this node
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

	// Process children
	for i, child := range node.Children {
		if err := i.indexNode(ctx, docID, child, path, i); err != nil {
			return err
		}
	}

	return nil
}

// Compile-time interface checks
var (
	_ ports.Parser     = (*ParserSvc)(nil)
	_ ports.Summarizer = (*SummarizerSvc)(nil)
	_ ports.Indexer    = (*IndexerSvc)(nil)
)