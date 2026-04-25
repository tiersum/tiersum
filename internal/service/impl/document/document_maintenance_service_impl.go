package document

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/metrics"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewDocumentMaintenanceService constructs the maintenance service implementation.
func NewDocumentMaintenanceService(
	docRepo storage.IDocumentRepository,
	chapterRepo storage.IChapterRepository,
	coldIndex storage.IColdIndex,
	deletedDocRepo storage.IDeletedDocumentRepository,
	persister IDocumentAnalysisPersister,
	analyzer IDocumentAnalysisGenerator,
	logger *zap.Logger,
) service.IDocumentMaintenanceService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &documentMaintenanceService{
		docRepo:        docRepo,
		chapterRepo:    chapterRepo,
		coldIndex:      coldIndex,
		deletedDocRepo: deletedDocRepo,
		persister:      persister,
		analyzer:       analyzer,
		logger:         logger,
	}
}

type documentMaintenanceService struct {
	docRepo          storage.IDocumentRepository
	chapterRepo      storage.IChapterRepository
	coldIndex        storage.IColdIndex
	deletedDocRepo   storage.IDeletedDocumentRepository
	persister        IDocumentAnalysisPersister
	analyzer         IDocumentAnalysisGenerator
	logger           *zap.Logger
	lastColdRefresh  time.Time
}

func (s *documentMaintenanceService) RunColdPromotionSweep(ctx context.Context) error {
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
				s.logger.Error("failed to promote document", zap.String("doc_id", doc.ID), zap.Error(err))
				continue
			}
			promotedCount++
		}
	}
	hotDocs, _ := s.docRepo.ListByStatus(ctx, types.DocStatusHot, 0)
	metrics.UpdateDocumentCount(string(types.DocStatusHot), len(hotDocs))

	s.logger.Info("document promotion job completed", zap.Int("checked", len(docs)), zap.Int("promoted", promotedCount))
	metrics.RecordJobExecution("document_promote", true, time.Since(start).Seconds())
	return nil
}

func (s *documentMaintenanceService) PromoteColdDocumentByID(ctx context.Context, docID string) error {
	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return err
	}
	if doc == nil || doc.Status != types.DocStatusCold {
		return nil
	}
	threshold := config.ColdPromotionThreshold()
	if doc.QueryCount < threshold {
		return nil
	}
	return s.promoteDocument(ctx, doc)
}

func (s *documentMaintenanceService) promoteDocument(ctx context.Context, doc *types.Document) error {
	s.logger.Info("promoting document to hot", zap.String("doc_id", doc.ID), zap.String("title", doc.Title), zap.Int("query_count", doc.QueryCount))

	if err := s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusWarming); err != nil {
		return err
	}
	analysis, err := s.analyzer.GenerateAnalysis(ctx, doc.Title, doc.Content)
	if err != nil {
		_ = s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusCold)
		return err
	}
	if err := s.persister.PersistAnalysis(ctx, doc, analysis); err != nil {
		_ = s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusCold)
		return err
	}
	if err := s.docRepo.UpdateStatus(ctx, doc.ID, types.DocStatusHot); err != nil {
		s.logger.Error("failed to update document status to hot", zap.String("doc_id", doc.ID), zap.Error(err))
	}

	// Remove promoted document from the cold index — it is no longer cold.
	if s.coldIndex != nil {
		if err := s.coldIndex.RemoveDocument(doc.ID); err != nil {
			s.logger.Warn("remove promoted doc from cold index", zap.String("doc_id", doc.ID), zap.Error(err))
		}
	}

	s.logger.Info("document promoted successfully", zap.String("doc_id", doc.ID), zap.Int("chapters", len(analysis.Chapters)))
	metrics.RecordDocumentPromotion(string(types.DocStatusCold), string(types.DocStatusHot))
	return nil
}

func (s *documentMaintenanceService) RecalculateDocumentHotScores(ctx context.Context) error {
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
			s.logger.Error("failed to update hot score", zap.String("doc_id", doc.ID), zap.Error(err))
			continue
		}
		updatedCount++
	}
	s.logger.Info("hot score update completed", zap.Int("total", len(docs)), zap.Int("updated", updatedCount))
	return nil
}

func (s *documentMaintenanceService) RefreshColdIndex(ctx context.Context) error {
	if s.coldIndex == nil || s.chapterRepo == nil {
		return nil
	}
	now := time.Now()
	since := s.lastColdRefresh
	if since.IsZero() {
		since = now // first run after startup/build: skip, RebuildFromDocuments is authoritative
	}
	s.lastColdRefresh = now

	// Phase 1: Process tombstone entries — remove deleted documents from the cold index.
	// s.lastColdRefresh advances on each run, so the same tombstone is never processed twice.
	if s.deletedDocRepo != nil {
		tombstones, err := s.deletedDocRepo.ListSince(ctx, since, 5000)
		if err != nil {
			return fmt.Errorf("list tombstones: %w", err)
		}
		for _, t := range tombstones {
			s.logger.Debug("removing deleted document from cold index", zap.String("doc_id", t.DocumentID))
			if err := s.coldIndex.RemoveDocument(t.DocumentID); err != nil {
				s.logger.Warn("remove deleted doc from cold index", zap.String("doc_id", t.DocumentID), zap.Error(err))
			}
		}
	}

	// Phase 2: Index new/updated cold documents from the chapters table.
	coldDocs, err := s.docRepo.ListByStatus(ctx, types.DocStatusCold, 5000)
	if err != nil {
		return fmt.Errorf("list cold docs: %w", err)
	}

	for i := range coldDocs {
		doc := &coldDocs[i]

		if doc.CreatedAt.Before(since) && doc.UpdatedAt.Before(since) {
			continue
		}
		if err := s.coldIndex.RemoveDocument(doc.ID); err != nil {
			s.logger.Warn("remove from cold index", zap.String("doc_id", doc.ID), zap.Error(err))
		}

		chapters, err := s.chapterRepo.ListByDocument(ctx, doc.ID)
		if err != nil {
			return fmt.Errorf("list chapters for %s: %w", doc.ID, err)
		}
		for _, ch := range chapters {
			if err := s.coldIndex.AddChapter(ctx, doc.ID, ch.Path, ch.Title, ch.Content); err != nil {
				s.logger.Error("add chapter to cold index",
					zap.String("doc_id", doc.ID),
					zap.String("path", ch.Path),
					zap.Error(err))
			}
		}
	}

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

var _ service.IDocumentMaintenanceService = (*documentMaintenanceService)(nil)

