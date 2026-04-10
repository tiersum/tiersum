package job

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/metrics"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// PromoteJob handles async promotion of cold documents to hot status
type PromoteJob struct {
	docRepo    storage.IDocumentRepository
	indexer    service.IIndexer
	summarizer service.ISummarizer
	logger     *zap.Logger
}

// NewPromoteJob creates a new document promotion job
func NewPromoteJob(
	docRepo storage.IDocumentRepository,
	indexer service.IIndexer,
	summarizer service.ISummarizer,
	logger *zap.Logger,
) *PromoteJob {
	return &PromoteJob{
		docRepo:    docRepo,
		indexer:    indexer,
		summarizer: summarizer,
		logger:     logger,
	}
}

// Name returns the job name
func (j *PromoteJob) Name() string {
	return "document_promote"
}

// Interval returns the execution interval
func (j *PromoteJob) Interval() time.Duration {
	return 5 * time.Minute
}

// Execute performs the promotion job
// Finds cold documents with query_count >= cold_promotion_threshold and promotes them to hot.
func (j *PromoteJob) Execute(ctx context.Context) error {
	start := time.Now()
	j.logger.Info("running document promotion job")

	threshold := config.ColdPromotionThreshold()

	// Find cold documents that need promotion
	docs, err := j.docRepo.ListByStatus(ctx, types.DocStatusCold, 100)
	if err != nil {
		j.logger.Error("failed to list cold documents", zap.Error(err))
		metrics.RecordJobExecution(j.Name(), false, time.Since(start).Seconds())
		return err
	}

	// Update cold document count metric
	metrics.UpdateDocumentCount(string(types.DocStatusCold), len(docs))

	var promotedCount int
	for _, doc := range docs {
		if doc.QueryCount >= threshold {
			if err := j.promoteDocument(ctx, &doc); err != nil {
				j.logger.Error("failed to promote document",
					zap.String("doc_id", doc.ID),
					zap.Error(err))
				continue
			}
			promotedCount++
		}
	}

	// Get hot document count
	hotDocs, _ := j.docRepo.ListByStatus(ctx, types.DocStatusHot, 0)
	metrics.UpdateDocumentCount(string(types.DocStatusHot), len(hotDocs))

	j.logger.Info("document promotion job completed",
		zap.Int("checked", len(docs)),
		zap.Int("promoted", promotedCount))

	metrics.RecordJobExecution(j.Name(), true, time.Since(start).Seconds())
	return nil
}

// PromoteByDocumentID promotes a single cold document if its query count meets the threshold.
func (j *PromoteJob) PromoteByDocumentID(ctx context.Context, docID string) error {
	doc, err := j.docRepo.GetByID(ctx, docID)
	if err != nil {
		return err
	}
	if doc == nil {
		return nil
	}
	if doc.Status != types.DocStatusCold {
		return nil
	}
	threshold := config.ColdPromotionThreshold()
	if doc.QueryCount < threshold {
		return nil
	}
	return j.promoteDocument(ctx, doc)
}

// promoteDocument promotes a single document from cold to hot status
func (j *PromoteJob) promoteDocument(ctx context.Context, doc *types.Document) error {
	j.logger.Info("promoting document to hot",
		zap.String("doc_id", doc.ID),
		zap.String("title", doc.Title),
		zap.Int("query_count", doc.QueryCount))

	// Set status to warming to prevent duplicate processing
	if err := j.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusWarming); err != nil {
		return err
	}

	// Perform full LLM analysis
	analysis, err := j.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
	if err != nil {
		// Revert status on failure
		j.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusCold)
		return err
	}

	// Index the document (creates summaries)
	if err := j.indexer.Index(ctx, doc, analysis); err != nil {
		// Revert status on failure
		j.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusCold)
		return err
	}

	// Update document status to hot
	if err := j.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusHot); err != nil {
		j.logger.Error("failed to update document status to hot",
			zap.String("doc_id", doc.ID),
			zap.Error(err))
		// Don't revert - the document is already indexed
	}

	j.logger.Info("document promoted successfully",
		zap.String("doc_id", doc.ID),
		zap.Int("chapters", len(analysis.Chapters)))

	// Record promotion metric
	metrics.RecordDocumentPromotion(string(types.DocStatusCold), string(types.DocStatusHot))

	return nil
}
