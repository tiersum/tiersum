package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
)

// IndexerJob periodically processes pending document indexing
type IndexerJob struct {
	docRepo     storage.IDocumentRepository
	summaryRepo storage.ISummaryRepository
	indexer     service.IIndexer
	logger      *zap.Logger
}

// NewIndexerJob creates a new indexer job
func NewIndexerJob(
	docRepo storage.IDocumentRepository,
	summaryRepo storage.ISummaryRepository,
	indexer service.IIndexer,
	logger *zap.Logger,
) *IndexerJob {
	return &IndexerJob{
		docRepo:     docRepo,
		summaryRepo: summaryRepo,
		indexer:     indexer,
		logger:      logger,
	}
}

// Name returns the job name
func (j *IndexerJob) Name() string {
	return "document_indexer"
}

// Interval returns the execution interval
func (j *IndexerJob) Interval() time.Duration {
	return 1 * time.Minute
}

// Execute performs the indexing job
func (j *IndexerJob) Execute(ctx context.Context) error {
	j.logger.Info("running document indexer job")
	// TODO: Implement actual indexing logic for pending documents
	return nil
}

// TagGroupJob periodically performs tag grouping
type TagGroupJob struct {
	groupingSvc service.ITagGroupService
	logger        *zap.Logger
}

// NewTagGroupJob creates a new tag grouping job
func NewTagGroupJob(
	groupingSvc service.ITagGroupService,
	logger *zap.Logger,
) *TagGroupJob {
	return &TagGroupJob{
		groupingSvc: groupingSvc,
		logger:      logger,
	}
}

// Name returns the job name
func (j *TagGroupJob) Name() string {
	return "tag_grouping"
}

// Interval returns the execution interval (30 minutes)
func (j *TagGroupJob) Interval() time.Duration {
	return 30 * time.Minute
}

// Execute performs the tag grouping job
// Strategy:
// 1. Check if grouping is needed (tag count changed or time elapsed)
// 2. If needed, perform LLM-based grouping
// 3. Update Level 1 groups and Level 2 tag assignments
func (j *TagGroupJob) Execute(ctx context.Context) error {
	j.logger.Info("running tag grouping job")

	// Check if refresh is needed
	shouldRefresh, err := j.groupingSvc.ShouldRefresh(ctx)
	if err != nil {
		j.logger.Error("failed to check if refresh needed", zap.Error(err))
		return err
	}

	if !shouldRefresh {
		j.logger.Info("tag grouping not needed at this time")
		return nil
	}

	// Perform grouping
	if err := j.groupingSvc.GroupTags(ctx); err != nil {
		j.logger.Error("failed to group tags", zap.Error(err))
		return err
	}

	j.logger.Info("tag grouping completed successfully")
	return nil
}
