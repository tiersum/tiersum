package indexer

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/core/parser"
	"github.com/tiersum/tiersum/internal/core/summarizer"
	"github.com/tiersum/tiersum/internal/storage"
)

// Indexer handles building and updating the hierarchical index
type Indexer struct {
	parser     *parser.Parser
	summarizer *summarizer.Summarizer
	storage    *storage.Storage
	logger     *zap.Logger
}

// New creates a new indexer
func New(summarizer *summarizer.Summarizer, store *storage.Storage, logger *zap.Logger) *Indexer {
	return &Indexer{
		parser:     parser.New(),
		summarizer: summarizer,
		storage:    store,
		logger:     logger,
	}
}

// IndexDocument indexes a new document with all summary tiers
func (i *Indexer) IndexDocument(ctx context.Context, docID uuid.UUID, content string) error {
	i.logger.Info("Indexing document", zap.String("doc_id", docID.String()))

	// Parse document structure
	doc, err := i.parser.Parse(content)
	if err != nil {
		return err
	}

	// Generate document-level summary
	docSummary, err := i.summarizer.SummarizeDocument(ctx, content)
	if err != nil {
		i.logger.Error("Failed to generate document summary", zap.Error(err))
		return err
	}

	// Store document summary
	docSummaryRecord := &storage.Summary{
		DocumentID: docID,
		Tier:       "document",
		Path:       "",
		Content:    docSummary,
	}
	if err := i.storage.CreateSummary(ctx, docSummaryRecord); err != nil {
		return err
	}

	// Process chapters recursively
	for _, chapter := range doc.Root.Children {
		if err := i.indexNode(ctx, docID, chapter); err != nil {
			return err
		}
	}

	i.logger.Info("Document indexed successfully", zap.String("doc_id", docID.String()))
	return nil
}

// indexNode recursively indexes a node and its children
func (i *Indexer) indexNode(ctx context.Context, docID uuid.UUID, node *parser.Node) error {
	// Generate chapter summary
	chapterSummary, err := i.summarizer.SummarizeChapter(ctx, node.Title, node.Content)
	if err != nil {
		i.logger.Error("Failed to generate chapter summary", zap.Error(err))
		return err
	}

	// Store chapter summary
	chapterSummaryRecord := &storage.Summary{
		DocumentID: docID,
		Tier:       "chapter",
		Path:       node.Path,
		Content:    chapterSummary,
	}
	if err := i.storage.CreateSummary(ctx, chapterSummaryRecord); err != nil {
		return err
	}

	// Process paragraphs
	paragraphs := i.parser.ExtractParagraphs(node.Content)
	for idx, para := range paragraphs {
		paraSummary, err := i.summarizer.SummarizeParagraph(ctx, para)
		if err != nil {
			i.logger.Error("Failed to generate paragraph summary", zap.Error(err))
			continue
		}

		paraPath := node.Path + "." + string(rune('0'+idx))
		paraSummaryRecord := &storage.Summary{
			DocumentID: docID,
			Tier:       "paragraph",
			Path:       paraPath,
			Content:    paraSummary,
		}
		if err := i.storage.CreateSummary(ctx, paraSummaryRecord); err != nil {
			return err
		}
	}

	// Recursively process sub-chapters
	for _, child := range node.Children {
		if err := i.indexNode(ctx, docID, child); err != nil {
			return err
		}
	}

	return nil
}

// Query performs a hierarchical query
func (i *Indexer) Query(ctx context.Context, question string, depth string) ([]QueryResult, error) {
	// TODO: Implement hierarchical query logic
	// This would:
	// 1. Start at document summaries
	// 2. Drill down based on relevance
	// 3. Return results at the requested depth
	return nil, nil
}

// QueryResult represents a query result
type QueryResult struct {
	DocumentID uuid.UUID `json:"document_id"`
	Tier       string    `json:"tier"`
	Path       string    `json:"path"`
	Content    string    `json:"content"`
	Score      float64   `json:"score"`
}
