package summarizer

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Provider defines the interface for LLM providers
type Provider interface {
	Summarize(ctx context.Context, content string, maxLength int) (string, error)
}

// Summarizer handles document summarization
type Summarizer struct {
	provider Provider
	logger   *zap.Logger
	config   Config
}

// Config holds summarizer configuration
type Config struct {
	DocumentMaxLength  int
	ChapterMaxLength   int
	ParagraphMaxLength int
	Workers            int
}

// New creates a new summarizer
func New(logger *zap.Logger) (*Summarizer, error) {
	config := Config{
		DocumentMaxLength:  viper.GetInt("summarization.document_summary_max"),
		ChapterMaxLength:   viper.GetInt("summarization.chapter_summary_max"),
		ParagraphMaxLength: viper.GetInt("summarization.paragraph_summary_max"),
		Workers:            viper.GetInt("summarization.workers"),
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
	if config.Workers == 0 {
		config.Workers = 4
	}

	providerType := viper.GetString("llm.provider")
	var provider Provider

	switch providerType {
	case "openai":
		provider = NewOpenAIProvider()
	case "anthropic":
		provider = NewAnthropicProvider()
	case "local":
		provider = NewLocalProvider()
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", providerType)
	}

	return &Summarizer{
		provider: provider,
		logger:   logger,
		config:   config,
	}, nil
}

// SummarizeDocument creates a document-level summary
func (s *Summarizer) SummarizeDocument(ctx context.Context, content string) (string, error) {
	s.logger.Debug("Summarizing document", zap.Int("content_length", len(content)))
	return s.provider.Summarize(ctx, content, s.config.DocumentMaxLength)
}

// SummarizeChapter creates a chapter-level summary
func (s *Summarizer) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	s.logger.Debug("Summarizing chapter", zap.String("title", title))
	fullContent := fmt.Sprintf("Chapter: %s\n\n%s", title, content)
	return s.provider.Summarize(ctx, fullContent, s.config.ChapterMaxLength)
}

// SummarizeParagraph creates a paragraph-level summary
func (s *Summarizer) SummarizeParagraph(ctx context.Context, content string) (string, error) {
	s.logger.Debug("Summarizing paragraph", zap.Int("content_length", len(content)))
	return s.provider.Summarize(ctx, content, s.config.ParagraphMaxLength)
}
