// Package svcimpl implements service layer interfaces
package svcimpl

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentSvc implements service.IDocumentService
type DocumentSvc struct {
	repo       storage.IDocumentRepository
	indexer    service.IIndexer
	summarizer service.ISummarizer
	topicSvc   service.ITopicService
	logger     *zap.Logger
}

// NewDocumentSvc creates a new document service
func NewDocumentSvc(repo storage.IDocumentRepository, indexer service.IIndexer, summarizer service.ISummarizer, topicSvc service.ITopicService, logger *zap.Logger) *DocumentSvc {
	return &DocumentSvc{
		repo:       repo,
		indexer:    indexer,
		summarizer: summarizer,
		topicSvc:   topicSvc,
		logger:     logger,
	}
}

// Ingest implements IDocumentService.Ingest
func (s *DocumentSvc) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error) {
	doc := &types.Document{
		Title:   req.Title,
		Content: req.Content,
		Format:  req.Format,
		Tags:    req.Tags,
	}

	// Use LLM to generate tags if not provided
	if len(doc.Tags) == 0 && s.summarizer != nil {
		s.logger.Info("generating tags via LLM", zap.String("title", doc.Title))
		analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
		if err != nil {
			s.logger.Warn("failed to analyze document, continuing without tags", zap.Error(err))
		} else {
			doc.Tags = analysis.Tags
			s.logger.Info("generated tags", zap.Strings("tags", doc.Tags))
		}
	}

	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, err
	}

	// Async indexing and topic matching
	if s.indexer != nil {
		go func() {
			if err := s.indexer.Index(context.Background(), doc.ID, doc.Content); err != nil {
				s.logger.Error("failed to index document", zap.String("id", doc.ID), zap.Error(err))
			}
		}()
	}

	// Auto-match document to existing topics based on tags
	if s.topicSvc != nil && len(doc.Tags) > 0 {
		go func() {
			added, err := s.topicSvc.AddDocumentToTopics(context.Background(), doc.ID, doc.Tags)
			if err != nil {
				s.logger.Warn("failed to add document to topics", zap.String("id", doc.ID), zap.Error(err))
			} else if added > 0 {
				s.logger.Info("auto-matched document to topics", zap.String("id", doc.ID), zap.Int("topics", added))
			}
		}()
	}

	return doc, nil
}

// Get implements IDocumentService.Get
func (s *DocumentSvc) Get(ctx context.Context, id string) (*types.Document, error) {
	return s.repo.GetByID(ctx, id)
}

// Compile-time interface check
var _ service.IDocumentService = (*DocumentSvc)(nil)
