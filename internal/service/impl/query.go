package impl

import (
	"context"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// QuerySvc implements service.IQueryService
type QuerySvc struct {
	summaryRepo storage.ISummaryRepository
	docRepo     storage.IDocumentRepository
	logger      *zap.Logger
}

// NewQuerySvc creates a new query service
func NewQuerySvc(docRepo storage.IDocumentRepository, summaryRepo storage.ISummaryRepository, logger *zap.Logger) *QuerySvc {
	return &QuerySvc{
		docRepo:     docRepo,
		summaryRepo: summaryRepo,
		logger:      logger,
	}
}

// Query implements IQueryService.Query
func (s *QuerySvc) Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error) {
	// TODO: Implement hierarchical query logic
	return nil, nil
}

var _ service.IQueryService = (*QuerySvc)(nil)
