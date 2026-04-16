package document

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewDocumentAnalysisPersister constructs the service.IDocumentAnalysisPersister implementation.
func NewDocumentAnalysisPersister(chapterRepo storage.IChapterRepository, docRepo storage.IDocumentRepository, logger *zap.Logger) service.IDocumentAnalysisPersister {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &chapterMaterializer{
		chapterRepo: chapterRepo,
		docRepo:     docRepo,
		logger:      logger,
	}
}

type chapterMaterializer struct {
	chapterRepo storage.IChapterRepository
	docRepo     storage.IDocumentRepository
	logger      *zap.Logger
}

func (m *chapterMaterializer) PersistAnalysis(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error {
	if doc == nil || analysis == nil {
		return nil
	}
	if m.docRepo != nil {
		if err := m.docRepo.UpdateSummary(ctx, doc.ID, analysis.Summary); err != nil {
			return fmt.Errorf("update document summary: %w", err)
		}
	}

	chRows := make([]types.Chapter, 0, len(analysis.Chapters))
	for i := range analysis.Chapters {
		ch := &analysis.Chapters[i]
		var path string
		if ch.Title == "" {
			path = fmt.Sprintf("%s/chapter/%d", doc.ID, i+1)
		} else {
			seg := sanitizePath(ch.Title)
			path = fmt.Sprintf("%s/%s", doc.ID, seg)
			if seg == "" || strings.TrimSpace(path) == doc.ID+"/" {
				path = fmt.Sprintf("%s/chapter/%d", doc.ID, i+1)
			}
		}
		chRows = append(chRows, types.Chapter{
			DocumentID: doc.ID,
			Path:       path,
			Title:      ch.Title,
			Summary:    ch.Summary,
			Content:    ch.Content,
		})
	}

	if m.chapterRepo != nil {
		if err := m.chapterRepo.ReplaceByDocument(ctx, doc.ID, chRows); err != nil {
			return fmt.Errorf("replace chapters: %w", err)
		}
	}

	m.logger.Info("materialized document analysis",
		zap.String("doc_id", doc.ID),
		zap.String("title", doc.Title),
		zap.Int("chapters", len(chRows)),
		zap.Int("tags", len(doc.Tags)),
	)
	return nil
}

func sanitizePath(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.TrimSpace(s)
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

var _ service.IDocumentAnalysisPersister = (*chapterMaterializer)(nil)

