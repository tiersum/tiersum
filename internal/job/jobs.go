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

// TagGroupingJob periodically performs tag clustering
type TagGroupingJob struct {
	clusteringSvc service.ITagGroupingService
	logger        *zap.Logger
}

// NewTagGroupingJob creates a new tag clustering job
func NewTagGroupingJob(
	clusteringSvc service.ITagGroupingService,
	logger *zap.Logger,
) *TagGroupingJob {
	return &TagGroupingJob{
		clusteringSvc: clusteringSvc,
		logger:        logger,
	}
}

// Name returns the job name
func (j *TagGroupingJob) Name() string {
	return "tag_clustering"
}

// Interval returns the execution interval (30 minutes)
func (j *TagGroupingJob) Interval() time.Duration {
	return 30 * time.Minute
}

// Execute performs the tag clustering job
// Strategy:
// 1. Check if clustering is needed (tag count changed or time elapsed)
// 2. If needed, perform LLM-based clustering
// 3. Update Level 1 clusters and Level 2 tag assignments
func (j *TagGroupingJob) Execute(ctx context.Context) error {
	j.logger.Info("running tag clustering job")

	// Check if refresh is needed
	shouldRefresh, err := j.clusteringSvc.ShouldRefresh(ctx)
	if err != nil {
		j.logger.Error("failed to check if refresh needed", zap.Error(err))
		return err
	}

	if !shouldRefresh {
		j.logger.Info("tag clustering not needed at this time")
		return nil
	}

	// Perform clustering
	if err := j.clusteringSvc.ClusterTags(ctx); err != nil {
		j.logger.Error("failed to cluster tags", zap.Error(err))
		return err
	}

	j.logger.Info("tag clustering completed successfully")
	return nil
}
