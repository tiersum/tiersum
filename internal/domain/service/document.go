// Package service implements business logic interfaces defined in ports
package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentSvc implements ports.DocumentService
type DocumentSvc struct {
	repo   ports.DocumentRepository
	indexer ports.Indexer
	logger *zap.Logger
}

// NewDocumentSvc creates a new document service
func NewDocumentSvc(repo ports.DocumentRepository, indexer ports.Indexer, logger *zap.Logger) *DocumentSvc {
	return &DocumentSvc{
		repo:   repo,
		indexer: indexer,
		logger: logger,
	}
}

// Ingest implements ports.DocumentService.Ingest
func (s *DocumentSvc) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error) {
	doc := &types.Document{
		Title:   req.Title,
		Content: req.Content,
		Format:  req.Format,
		Tags:    req.Tags,
	}

	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, err
	}

	// Async indexing
	if s.indexer != nil {
		go func() {
			if err := s.indexer.Index(context.Background(), doc.ID, doc.Content); err != nil {
				s.logger.Error("failed to index document", zap.String("id", doc.ID), zap.Error(err))
			}
		}()
	}

	return doc, nil
}

// Get implements ports.DocumentService.Get
func (s *DocumentSvc) Get(ctx context.Context, id string) (*types.Document, error) {
	return s.repo.GetByID(ctx, id)
}

// QuerySvc implements ports.QueryService
type QuerySvc struct {
	summaryRepo ports.SummaryRepository
	docRepo     ports.DocumentRepository
	logger      *zap.Logger
}

// NewQuerySvc creates a new query service
func NewQuerySvc(docRepo ports.DocumentRepository, summaryRepo ports.SummaryRepository, logger *zap.Logger) *QuerySvc {
	return &QuerySvc{
		docRepo:     docRepo,
		summaryRepo: summaryRepo,
		logger:      logger,
	}
}

// Query implements ports.QueryService.Query
func (s *QuerySvc) Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error) {
	// TODO: Implement hierarchical query logic
	// This is a simplified version
	return nil, nil
}

// Compile-time interface checks
var (
	_ ports.DocumentService = (*DocumentSvc)(nil)
	_ ports.QueryService    = (*QuerySvc)(nil)
)