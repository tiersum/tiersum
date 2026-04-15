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

// NewChapterMaterializer constructs the service.IChapterMaterializer implementation.
func NewChapterMaterializer(chapterRepo storage.IChapterRepository, docRepo storage.IDocumentRepository, logger *zap.Logger) service.IChapterMaterializer {
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

func (m *chapterMaterializer) Materialize(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error {
	// Persist document-level summary on the document row (used by the UI and hot read paths).
	if m.docRepo != nil {
		if err := m.docRepo.UpdateSummary(ctx, doc.ID, analysis.Summary); err != nil {
			return fmt.Errorf("update document summary: %w", err)
		}
	}

	chRows := make([]types.Chapter, 0, len(analysis.Chapters))
	used := make(map[string]int)
	for _, ch := range analysis.Chapters {
		slug := sanitizePath(strings.TrimSpace(ch.Title))
		if slug == "" {
			slug = "chapter"
		}
		used[slug]++
		path := doc.ID + "/" + slug
		if used[slug] > 1 {
			path = fmt.Sprintf("%s/%s-%d", doc.ID, slug, used[slug])
		}
		chRows = append(chRows, types.Chapter{
			DocumentID: doc.ID,
			Path:       path,
			Title:      strings.TrimSpace(ch.Title),
			Summary:    strings.TrimSpace(ch.Summary),
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
		zap.Int("chapters", len(analysis.Chapters)),
		zap.Int("tags", len(doc.Tags)))

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

var _ service.IChapterMaterializer = (*chapterMaterializer)(nil)
