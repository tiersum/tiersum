package svcimpl

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// IndexerSvc implements service.IIndexer
type IndexerSvc struct {
	summarizer service.ISummarizer
	summaryRepo storage.ISummaryRepository
	logger     *zap.Logger
}

// NewIndexerSvc creates a new indexer service
func NewIndexerSvc(summarizer service.ISummarizer, summaryRepo storage.ISummaryRepository, logger *zap.Logger) *IndexerSvc {
	return &IndexerSvc{
		summarizer: summarizer,
		summaryRepo: summaryRepo,
		logger:     logger,
	}
}

// Index implements IIndexer.Index
// Creates document summary, chapter summaries with content
func (i *IndexerSvc) Index(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error {
	// Clear existing summaries for this document (for re-indexing)
	if err := i.summaryRepo.DeleteByDocument(ctx, doc.ID); err != nil {
		i.logger.Warn("failed to delete existing summaries", zap.String("doc_id", doc.ID), zap.Error(err))
	}

	// Store document-level summary
	docSum := &types.Summary{
		DocumentID: doc.ID,
		Tier:       types.TierDocument,
		Path:       doc.ID,
		Content:    analysis.Summary,
		IsSource:   false,
	}
	if err := i.summaryRepo.Create(ctx, docSum); err != nil {
		return fmt.Errorf("create document summary: %w", err)
	}

	// Store chapter summaries with source content
	for _, chapter := range analysis.Chapters {
		chapterPath := fmt.Sprintf("%s/%s", doc.ID, sanitizePath(chapter.Title))

		chapterSum := &types.Summary{
			DocumentID: doc.ID,
			Tier:       types.TierChapter,
			Path:       chapterPath,
			Content:    chapter.Summary,
			IsSource:   false,
		}
		if err := i.summaryRepo.Create(ctx, chapterSum); err != nil {
			return fmt.Errorf("create chapter summary: %w", err)
		}

		// Store chapter source content separately
		// We store it with a /source suffix for retrieval
		sourceSum := &types.Summary{
			DocumentID: doc.ID,
			Tier:       types.TierSource,
			Path:       chapterPath + "/source",
			Content:    chapter.Content,
			IsSource:   true,
		}
		if err := i.summaryRepo.Create(ctx, sourceSum); err != nil {
			return fmt.Errorf("create chapter source: %w", err)
		}
	}

	i.logger.Info("indexed document",
		zap.String("doc_id", doc.ID),
		zap.String("title", doc.Title),
		zap.Int("chapters", len(analysis.Chapters)),
		zap.Int("tags", len(doc.Tags)))

	return nil
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
