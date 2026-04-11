package svcimpl

import (
	"context"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/metrics"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// DocumentMaintenanceSvc implements service.IDocumentMaintenanceService for scheduled jobs.
type DocumentMaintenanceSvc struct {
	docRepo    storage.IDocumentRepository
	indexer    service.IIndexer
	summarizer service.ISummarizer
	logger     *zap.Logger
}

// NewDocumentMaintenanceSvc wires cold promotion and hot-score maintenance.
func NewDocumentMaintenanceSvc(
	docRepo storage.IDocumentRepository,
	indexer service.IIndexer,
	summarizer service.ISummarizer,
	logger *zap.Logger,
) *DocumentMaintenanceSvc {
	return &DocumentMaintenanceSvc{
		docRepo:    docRepo,
		indexer:    indexer,
		summarizer: summarizer,
		logger:     logger,
	}
}

// RunColdPromotionSweep implements IDocumentMaintenanceService (scheduled sweep).
func (s *DocumentMaintenanceSvc) RunColdPromotionSweep(ctx context.Context) error {
	start := time.Now()
	s.logger.Info("running document promotion job")

	threshold := config.ColdPromotionThreshold()

	docs, err := s.docRepo.ListByStatus(ctx, types.DocStatusCold, 100)
	if err != nil {
		s.logger.Error("failed to list cold documents", zap.Error(err))
		metrics.RecordJobExecution("document_promote", false, time.Since(start).Seconds())
		return err
	}

	metrics.UpdateDocumentCount(string(types.DocStatusCold), len(docs))

	var promotedCount int
	for i := range docs {
		doc := &docs[i]
		if doc.QueryCount >= threshold {
			if err := s.promoteDocument(ctx, doc); err != nil {
				s.logger.Error("failed to promote document",
					zap.String("doc_id", doc.ID),
					zap.Error(err))
				continue
			}
			promotedCount++
		}
	}

	hotDocs, _ := s.docRepo.ListByStatus(ctx, types.DocStatusHot, 0)
	metrics.UpdateDocumentCount(string(types.DocStatusHot), len(hotDocs))

	s.logger.Info("document promotion job completed",
		zap.Int("checked", len(docs)),
		zap.Int("promoted", promotedCount))

	metrics.RecordJobExecution("document_promote", true, time.Since(start).Seconds())
	return nil
}

// PromoteColdDocumentByID implements IDocumentMaintenanceService (async queue).
func (s *DocumentMaintenanceSvc) PromoteColdDocumentByID(ctx context.Context, docID string) error {
	doc, err := s.docRepo.GetByID(ctx, docID)
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
	return s.promoteDocument(ctx, doc)
}

func (s *DocumentMaintenanceSvc) promoteDocument(ctx context.Context, doc *types.Document) error {
	s.logger.Info("promoting document to hot",
		zap.String("doc_id", doc.ID),
		zap.String("title", doc.Title),
		zap.Int("query_count", doc.QueryCount))

	if err := s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusWarming); err != nil {
		return err
	}

	analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
	if err != nil {
		_ = s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusCold)
		return err
	}

	if err := s.indexer.Index(ctx, doc, analysis); err != nil {
		_ = s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusCold)
		return err
	}

	if err := s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusHot); err != nil {
		s.logger.Error("failed to update document status to hot",
			zap.String("doc_id", doc.ID),
			zap.Error(err))
	}

	s.logger.Info("document promoted successfully",
		zap.String("doc_id", doc.ID),
		zap.Int("chapters", len(analysis.Chapters)))

	metrics.RecordDocumentPromotion(string(types.DocStatusCold), string(types.DocStatusHot))

	return nil
}

// RecalculateDocumentHotScores implements IDocumentMaintenanceService.
func (s *DocumentMaintenanceSvc) RecalculateDocumentHotScores(ctx context.Context) error {
	s.logger.Info("running hot score update job")

	docs, err := s.docRepo.ListAll(ctx, 10000)
	if err != nil {
		s.logger.Error("failed to list documents", zap.Error(err))
		return err
	}

	now := time.Now()
	var updatedCount int

	for i := range docs {
		doc := &docs[i]
		score := calculateHotScore(doc.QueryCount, doc.LastQueryAt, now)
		if err := s.docRepo.UpdateHotScore(ctx, doc.ID, score); err != nil {
			s.logger.Error("failed to update hot score",
				zap.String("doc_id", doc.ID),
				zap.Error(err))
			continue
		}
		updatedCount++
	}

	s.logger.Info("hot score update completed",
		zap.Int("total", len(docs)),
		zap.Int("updated", updatedCount))
	return nil
}

func calculateHotScore(queryCount int, lastQueryAt *time.Time, now time.Time) float64 {
	if queryCount == 0 {
		return 0
	}

	hoursSinceQuery := 168.0
	if lastQueryAt != nil {
		hoursSinceQuery = now.Sub(*lastQueryAt).Hours()
		if hoursSinceQuery < 0 {
			hoursSinceQuery = 0
		}
	}

	score := float64(queryCount) / (1 + math.Log1p(hoursSinceQuery))
	return math.Round(score*100) / 100
}

var _ service.IDocumentMaintenanceService = (*DocumentMaintenanceSvc)(nil)
