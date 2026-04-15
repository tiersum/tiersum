package document

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewHotIngestProcessor constructs the IHotIngestProcessor implementation.
func NewHotIngestProcessor(
	docRepo storage.IDocumentRepository,
	analyzer service.IDocumentAnalyzer,
	materializer service.IChapterMaterializer,
	tagRepo storage.ITagRepository,
	logger *zap.Logger,
) service.IHotIngestProcessor {
	return &hotIngestProcessor{
		docRepo:      docRepo,
		analyzer:     analyzer,
		materializer: materializer,
		tagRepo:      tagRepo,
		logger:       logger,
	}
}

type hotIngestProcessor struct {
	docRepo      storage.IDocumentRepository
	analyzer     service.IDocumentAnalyzer
	materializer service.IChapterMaterializer
	tagRepo      storage.ITagRepository
	logger       *zap.Logger
}

func (p *hotIngestProcessor) ProcessHotIngest(ctx context.Context, work types.HotIngestWork) error {
	return processHotIngest(ctx, work, p.docRepo, p.analyzer, p.materializer, p.tagRepo, p.logger)
}

func processHotIngest(
	ctx context.Context,
	work types.HotIngestWork,
	docRepo storage.IDocumentRepository,
	analyzer service.IDocumentAnalyzer,
	materializer service.IChapterMaterializer,
	tagRepo storage.ITagRepository,
	logger *zap.Logger,
) error {
	if work.DocID == "" {
		return nil
	}
	doc, err := docRepo.GetByID(ctx, work.DocID)
	if err != nil {
		return err
	}
	if doc == nil {
		logger.Warn("hot ingest: document not found", zap.String("doc_id", work.DocID))
		return nil
	}
	if doc.Status != types.DocStatusHot {
		logger.Info("hot ingest: skipping non-hot document",
			zap.String("doc_id", work.DocID),
			zap.String("status", string(doc.Status)))
		return nil
	}

	prebuilt := work.PrebuiltTags
	analysis, err := analyzer.AnalyzeDocument(ctx, doc.Title, doc.Content)
	if err != nil {
		logger.Warn("failed to analyze document (async)", zap.String("doc_id", doc.ID), zap.Error(err))
		analysis = &types.DocumentAnalysisResult{
			Summary:  truncateContent(doc.Content, 200),
			Tags:     append([]string(nil), prebuilt...),
			Chapters: []types.ChapterInfo{},
		}
	}

	var mergedTags []string
	if len(prebuilt) > 0 {
		tagSet := make(map[string]bool)
		for _, tag := range prebuilt {
			tagSet[tag] = true
		}
		for _, tag := range analysis.Tags {
			tagSet[tag] = true
		}
		mergedTags = make([]string, 0, len(tagSet))
		for tag := range tagSet {
			mergedTags = append(mergedTags, tag)
		}
		analysis.Tags = mergedTags
	} else {
		mergedTags = analysis.Tags
	}

	if err := docRepo.UpdateTags(ctx, doc.ID, mergedTags); err != nil {
		return fmt.Errorf("update tags after async analysis: %w", err)
	}
	doc.Tags = mergedTags

	if err := materializer.Materialize(ctx, doc, analysis); err != nil {
		logger.Error("failed to materialize document (async)", zap.String("doc_id", doc.ID), zap.Error(err))
		return err
	}

	for _, tag := range doc.Tags {
		tagEntity := &types.Tag{ID: uuid.New().String(), Name: tag, TopicID: ""}
		if err := tagRepo.Create(ctx, tagEntity); err != nil {
			logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
		} else if err := tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
			logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
		}
	}

	return nil
}

func truncateContent(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var _ service.IHotIngestProcessor = (*hotIngestProcessor)(nil)
