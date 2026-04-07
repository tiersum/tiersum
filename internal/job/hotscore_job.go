package job

import (
	"context"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
)

// HotScoreJob periodically updates hot scores for all documents
type HotScoreJob struct {
	docRepo storage.IDocumentRepository
	logger  *zap.Logger
}

// NewHotScoreJob creates a new hot score calculation job
func NewHotScoreJob(
	docRepo storage.IDocumentRepository,
	logger *zap.Logger,
) *HotScoreJob {
	return &HotScoreJob{
		docRepo: docRepo,
		logger:  logger,
	}
}

// Name returns the job name
func (j *HotScoreJob) Name() string {
	return "hot_score_update"
}

// Interval returns the execution interval (runs every hour)
func (j *HotScoreJob) Interval() time.Duration {
	return 1 * time.Hour
}

// Execute performs the hot score calculation
// Formula: hot_score = query_count / (1 + hours_since_last_query)
// This gives higher scores to documents that are queried frequently and recently
func (j *HotScoreJob) Execute(ctx context.Context) error {
	j.logger.Info("running hot score update job")

	// Get all documents
	docs, err := j.docRepo.ListAll(ctx, 10000)
	if err != nil {
		j.logger.Error("failed to list documents", zap.Error(err))
		return err
	}

	now := time.Now()
	var updatedCount int

	for _, doc := range docs {
		// Calculate hot score
		score := j.calculateHotScore(doc.QueryCount, doc.LastQueryAt, now)

		// Update the score
		if err := j.docRepo.UpdateHotScore(ctx, doc.ID, score); err != nil {
			j.logger.Error("failed to update hot score",
				zap.String("doc_id", doc.ID),
				zap.Error(err))
			continue
		}
		updatedCount++
	}

	j.logger.Info("hot score update completed",
		zap.Int("total", len(docs)),
		zap.Int("updated", updatedCount))
	return nil
}

// calculateHotScore calculates the hot score for a document
// Formula: query_count / (1 + hours_since_last_query)
// - Documents with more queries get higher scores
// - Documents queried recently get higher scores (time decay)
func (j *HotScoreJob) calculateHotScore(queryCount int, lastQueryAt *time.Time, now time.Time) float64 {
	if queryCount == 0 {
		return 0
	}

	// Default to a large time gap if never queried
	hoursSinceQuery := 168.0 // 1 week in hours

	if lastQueryAt != nil {
		hoursSinceQuery = now.Sub(*lastQueryAt).Hours()
		if hoursSinceQuery < 0 {
			hoursSinceQuery = 0
		}
	}

	// Apply formula: query_count / (1 + hours_since_last_query)
	// Add 1 to denominator to avoid division by zero and reduce impact of very recent queries
	score := float64(queryCount) / (1 + math.Log1p(hoursSinceQuery))

	// Round to 2 decimal places
	return math.Round(score*100) / 100
}
