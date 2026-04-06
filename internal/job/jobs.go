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
	topicRepo  storage.ITopicSummaryRepository
	docRepo    storage.IDocumentRepository
	summarizer service.ISummarizer
	logger     *zap.Logger
}

// NewTopicAggregatorJob creates a new topic aggregator job
func NewTopicAggregatorJob(
	topicRepo storage.ITopicSummaryRepository,
	docRepo storage.IDocumentRepository,
	summarizer service.ISummarizer,
	logger *zap.Logger,
) *TopicAggregatorJob {
	return &TopicAggregatorJob{
		topicRepo:  topicRepo,
		docRepo:    docRepo,
		summarizer: summarizer,
		logger:     logger,
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
func (j *TopicAggregatorJob) Execute(ctx context.Context) error {
	j.logger.Info("running topic aggregator job")
	// TODO: Implement actual aggregation logic
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
