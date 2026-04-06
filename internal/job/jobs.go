package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/ports"
)

// IndexerJob periodically processes pending document indexing
type IndexerJob struct {
	docRepo    ports.DocumentRepository
	summaryRepo ports.SummaryRepository
	indexer    ports.Indexer
	logger     *zap.Logger
}

// NewIndexerJob creates a new indexer job
func NewIndexerJob(
	docRepo ports.DocumentRepository,
	summaryRepo ports.SummaryRepository,
	indexer ports.Indexer,
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
	return 1 * time.Minute // Run every minute
}

// Execute performs the indexing job
func (j *IndexerJob) Execute(ctx context.Context) error {
	j.logger.Info("running document indexer job")

	// TODO: Implement actual indexing logic
	// 1. Query documents without summaries
	// 2. Process each document through indexer
	// 3. Mark as processed

	return nil
}

// TopicAggregatorJob aggregates documents into topics based on tags
type TopicAggregatorJob struct {
	topicRepo  ports.TopicSummaryRepository
	docRepo    ports.DocumentRepository
	summarizer ports.Summarizer
	logger     *zap.Logger
}

// NewTopicAggregatorJob creates a new topic aggregator job
func NewTopicAggregatorJob(
	topicRepo ports.TopicSummaryRepository,
	docRepo ports.DocumentRepository,
	summarizer ports.Summarizer,
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
	return 5 * time.Minute // Run every 5 minutes
}

// Execute performs the topic aggregation job
func (j *TopicAggregatorJob) Execute(ctx context.Context) error {
	j.logger.Info("running topic aggregator job")

	// TODO: Implement actual aggregation logic
	// 1. Group documents by common tags
	// 2. For each group with >1 documents, generate/update topic summary
	// 3. Update topic_summaries table

	return nil
}

// CacheCleanupJob cleans up expired cache entries
type CacheCleanupJob struct {
	cache  ports.Cache
	logger *zap.Logger
}

// NewCacheCleanupJob creates a new cache cleanup job
func NewCacheCleanupJob(cache ports.Cache, logger *zap.Logger) *CacheCleanupJob {
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
	return 10 * time.Minute // Run every 10 minutes
}

// Execute performs the cache cleanup
func (j *CacheCleanupJob) Execute(ctx context.Context) error {
	j.logger.Info("running cache cleanup job")

	// TODO: Implement cache cleanup logic
	// Clear expired cache entries

	return nil
}
