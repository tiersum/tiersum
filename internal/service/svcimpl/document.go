package svcimpl

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentSvc implements service.IDocumentService
type DocumentSvc struct {
	docRepo       storage.IDocumentRepository
	indexer       service.IIndexer
	summarizer    service.ISummarizer
	tagRepo storage.ITagRepository
	logger        *zap.Logger
}

// NewDocumentSvc creates a new document service
func NewDocumentSvc(
	docRepo storage.IDocumentRepository,
	indexer service.IIndexer,
	summarizer service.ISummarizer,
	tagRepo storage.ITagRepository,
	logger *zap.Logger,
) *DocumentSvc {
	return &DocumentSvc{
		docRepo:       docRepo,
		indexer:       indexer,
		summarizer:    summarizer,
		tagRepo: tagRepo,
		logger:        logger,
	}
}

// Ingest implements IDocumentService.Ingest
func (s *DocumentSvc) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
	// Create document entity
	doc := &types.Document{
		ID:      uuid.New().String(),
		Title:   req.Title,
		Content: req.Content,
		Format:  req.Format,
	}

	// Analyze document with LLM (if no tags provided)
	if len(req.Tags) == 0 {
		analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
		if err != nil {
			s.logger.Warn("failed to analyze document", zap.Error(err))
			// Continue with empty analysis
			analysis = &types.DocumentAnalysisResult{
				Summary:  truncateContent(doc.Content, 200),
				Tags:     []string{},
				Chapters: []types.ChapterInfo{},
			}
		}

		doc.Tags = analysis.Tags

		// Index the document with analysis results
		if err := s.indexer.Index(ctx, doc, analysis); err != nil {
			s.logger.Error("failed to index document", zap.Error(err))
			// Continue even if indexing fails
		}
	} else {
		doc.Tags = req.Tags

		// Analyze anyway to get chapters and summary
		analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
		if err == nil {
			// Merge provided tags with generated tags (deduplicate)
			tagSet := make(map[string]bool)
			for _, tag := range req.Tags {
				tagSet[tag] = true
			}
			for _, tag := range analysis.Tags {
				tagSet[tag] = true
			}
			mergedTags := make([]string, 0, len(tagSet))
			for tag := range tagSet {
				mergedTags = append(mergedTags, tag)
			}
			doc.Tags = mergedTags

			// Index with analysis
			if err := s.indexer.Index(ctx, doc, analysis); err != nil {
				s.logger.Error("failed to index document", zap.Error(err))
			}
		}
	}

	// Save document
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt
	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	// Update global tags
	for _, tag := range doc.Tags {
		tag := &types.Tag{
			ID:        uuid.New().String(),
			Name:      tag,
			GroupID: "", // Will be assigned by clustering service
		}
		if err := s.tagRepo.Create(ctx, tag); err != nil {
			s.logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
		} else {
			// Increment document count
			if err := s.tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
				s.logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
			}
		}
	}

	s.logger.Info("document ingested",
		zap.String("id", doc.ID),
		zap.String("title", doc.Title),
		zap.Int("tags", len(doc.Tags)))

	return &types.CreateDocumentResponse{
		ID:           doc.ID,
		Title:        doc.Title,
		Format:       doc.Format,
		Tags:         doc.Tags,
		Summary:      "", // Could fetch from summary repo
		ChapterCount: 0,  // Could fetch from summary repo
		CreatedAt:    doc.CreatedAt,
	}, nil
}

// Get implements IDocumentService.Get
func (s *DocumentSvc) Get(ctx context.Context, id string) (*types.Document, error) {
	return s.docRepo.GetByID(ctx, id)
}

// truncateContent truncates content to max length
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

var _ service.IDocumentService = (*DocumentSvc)(nil)
