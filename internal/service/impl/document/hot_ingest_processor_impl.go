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

// failureChapterSummaryMax caps only the synthetic virtual failure chapter summary (not LLM output).
const failureChapterSummaryMax = 100

// NewHotIngestProcessor constructs the IHotIngestProcessor implementation.
func NewHotIngestProcessor(
	docRepo storage.IDocumentRepository,
	analyzer service.IDocumentAnalysisGenerator,
	persister service.IDocumentAnalysisPersister,
	tagRepo storage.ITagRepository,
	logger *zap.Logger,
) service.IHotIngestProcessor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &hotIngestProcessor{
		docRepo:   docRepo,
		analyzer:  analyzer,
		persister: persister,
		tagRepo:   tagRepo,
		logger:    logger,
	}
}

type hotIngestProcessor struct {
	docRepo   storage.IDocumentRepository
	analyzer  service.IDocumentAnalysisGenerator
	persister service.IDocumentAnalysisPersister
	tagRepo   storage.ITagRepository
	logger    *zap.Logger
}

func (p *hotIngestProcessor) ProcessHotIngest(ctx context.Context, work types.HotIngestWork) error {
	if work.DocID == "" {
		return nil
	}
	if p.logger != nil {
		p.logger.Info("hot ingest: processing", zap.String("doc_id", work.DocID))
	}
	doc, err := p.docRepo.GetByID(ctx, work.DocID)
	if err != nil {
		return err
	}
	if doc == nil {
		p.logger.Warn("hot ingest: document not found", zap.String("doc_id", work.DocID))
		return nil
	}
	if doc.Status != types.DocStatusHot {
		p.logger.Info("hot ingest: skipping non-hot document",
			zap.String("doc_id", work.DocID),
			zap.String("status", string(doc.Status)))
		return nil
	}

	prebuilt := work.PrebuiltTags
	analysis, err := p.analyzer.GenerateAnalysis(ctx, doc.Title, doc.Content)
	if err != nil {
		p.logger.Warn("hot ingest: analyze failed, writing failure chapter", zap.String("doc_id", doc.ID), zap.Error(err))
		analysis = analysisFailureResult(doc.Title, err)
		analysis.Tags = append([]string(nil), prebuilt...)
	}

	mergedTags := mergeTags(prebuilt, analysis.Tags)
	analysis.Tags = mergedTags
	if err := p.docRepo.UpdateTags(ctx, doc.ID, mergedTags); err != nil {
		return fmt.Errorf("update tags after async analysis: %w", err)
	}
	doc.Tags = mergedTags

	if err := p.persister.PersistAnalysis(ctx, doc, analysis); err != nil {
		p.logger.Error("hot ingest: materialize failed", zap.String("doc_id", doc.ID), zap.Error(err))
		return err
	}

	prebuiltLower := make(map[string]struct{}, len(work.PrebuiltTags))
	for _, t := range work.PrebuiltTags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		prebuiltLower[strings.ToLower(t)] = struct{}{}
	}

	for _, tag := range doc.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, alreadyCounted := prebuiltLower[strings.ToLower(tag)]; alreadyCounted {
			continue
		}
		tagEntity := &types.Tag{ID: uuid.New().String(), Name: tag, TopicID: ""}
		if err := p.tagRepo.Create(ctx, tagEntity); err != nil {
			p.logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
		} else if err := p.tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
			p.logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
		}
	}

	return nil
}

func analysisFailureResult(title string, err error) *types.DocumentAnalysisResult {
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = "unknown error"
	}
	summary := truncateString("Document analysis failed: "+msg, failureChapterSummaryMax)
	// Persist the error once as a single virtual chapter so operators can see the failure in UI and via GET /chapters.
	return &types.DocumentAnalysisResult{
		Summary: "",
		Tags:    []string{},
		Chapters: []types.ChapterInfo{{
			Title:   "[analysis failed] " + titleOrDefault(title),
			Summary: summary,
			Content: "Document analysis failed.\n\nError:\n" + truncateString(msg, 4000),
		}},
	}
}

func mergeTags(prebuilt, generated []string) []string {
	// Preserve exact tag strings from the LLM (and prebuilt request tags); dedupe by string identity only.
	set := make(map[string]struct{}, len(prebuilt)+len(generated))
	out := make([]string, 0, len(prebuilt)+len(generated))
	push := func(t string) {
		if t == "" {
			return
		}
		if _, ok := set[t]; ok {
			return
		}
		set[t] = struct{}{}
		out = append(out, t)
	}
	for _, t := range prebuilt {
		push(t)
	}
	for _, t := range generated {
		push(t)
	}
	return out
}

var _ service.IHotIngestProcessor = (*hotIngestProcessor)(nil)
