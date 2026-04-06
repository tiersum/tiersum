// Package core implements domain logic interfaces defined in ports
package core

import (
	"context"
	"fmt"

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