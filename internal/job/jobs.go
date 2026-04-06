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
	// TODO: Implement actual indexing logic
	return nil
}

// TopicAggregatorJob aggregates documents into topics based on tags
type TopicAggregatorJob struct {
	topicSvc service.ITopicService
	logger   *zap.Logger
}

// NewTopicAggregatorJob creates a new topic aggregator job
func NewTopicAggregatorJob(
	topicSvc service.ITopicService,
	logger *zap.Logger,
) *TopicAggregatorJob {
	return &TopicAggregatorJob{
		topicSvc: topicSvc,
		logger:   logger,
	}
}

// Name returns the job name
func (j *TopicAggregatorJob) Name() string {
	return "topic_aggregator"
}

// Interval returns the execution interval
func (j *TopicAggregatorJob) Interval() time.Duration {
	return 5 * time.Minute
}

// Execute performs the topic aggregation job
// Strategy:
// 1. Scan all existing topics and their tags
// 2. For topics with insufficient documents (< 3), try to find more matching documents
// 3. Create new topics from orphaned documents that share common tags
func (j *TopicAggregatorJob) Execute(ctx context.Context) error {
	j.logger.Info("running topic aggregator job")

	// Get all existing topics
	topics, err := j.topicSvc.ListTopics(ctx)
	if err != nil {
		return err
	}

	// Log current state
	totalDocs := 0
	for _, topic := range topics {
		totalDocs += len(topic.DocumentIDs)
	}
	j.logger.Info("topic aggregation stats",
		zap.Int("topics", len(topics)),
		zap.Int("total_doc_refs", totalDocs))

	// TODO: Implement automatic topic creation from common tags
	// This would require:
	// 1. Query all documents and extract tag frequency
	// 2. Find tag combinations that appear in 3+ documents
	// 3. Create topics for those combinations

	return nil
}

// CacheCleanupJob cleans up expired cache entries
type CacheCleanupJob struct {
	cache  storage.ICache
	logger *zap.Logger
}

// NewCacheCleanupJob creates a new cache cleanup job
func NewCacheCleanupJob(cache storage.ICache, logger *zap.Logger) *CacheCleanupJob {
	return &CacheCleanupJob{
		cache:  cache,
		logger: logger,
	}
}

// Name returns the job name
func (j *CacheCleanupJob) Name() string {
	return "cache_cleanup"
}

// Interval returns the execution interval
func (j *CacheCleanupJob) Interval() time.Duration {
	return 10 * time.Minute
}

// Execute performs the cache cleanup
func (j *CacheCleanupJob) Execute(ctx context.Context) error {
	j.logger.Info("running cache cleanup job")
	// TODO: Implement cache cleanup logic
	return nil
}
