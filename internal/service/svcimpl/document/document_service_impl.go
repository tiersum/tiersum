package document

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewDocumentService constructs the IDocumentService implementation.
func NewDocumentService(
	docRepo storage.IDocumentRepository,
	materializer service.IChapterMaterializer,
	analyzer service.IDocumentAnalyzer,
	tagRepo storage.ITagRepository,
	coldIndex storage.IColdIndex,
	quotaManager interface{ CheckAndConsume() bool },
	logger *zap.Logger,
	hotIngestQueue chan<- types.HotIngestWork,
) service.IDocumentService {
	return &documentService{
		docRepo:        docRepo,
		materializer:   materializer,
		analyzer:       analyzer,
		tagRepo:        tagRepo,
		coldIndex:      coldIndex,
		quotaManager:   quotaManager,
		logger:         logger,
		hotIngestQueue: hotIngestQueue,
	}
}

type documentService struct {
	docRepo        storage.IDocumentRepository
	materializer   service.IChapterMaterializer
	analyzer       service.IDocumentAnalyzer
	tagRepo        storage.ITagRepository
	coldIndex      storage.IColdIndex
	quotaManager   interface{ CheckAndConsume() bool }
	logger         *zap.Logger
	hotIngestQueue chan<- types.HotIngestWork
}

func shouldBeHot(content string, mode string, hasPrebuiltSummary bool, quota interface{ CheckAndConsume() bool }) bool {
	m := types.NormalizeDocumentIngestMode(mode)
	switch m {
	case types.DocumentIngestModeHot:
		return true
	case types.DocumentIngestModeCold:
		return false
	default: // auto
		if hasPrebuiltSummary {
			return true
		}
		if quota == nil || !quota.CheckAndConsume() {
			return false
		}
		return len(content) > config.HotContentThreshold()
	}
}

func validateIngestRequest(req types.CreateDocumentRequest) error {
	maxBytes := config.DocumentMaxBodyBytes()
	if maxBytes > 0 && int64(len(req.Content)) > maxBytes {
		return fmt.Errorf("content exceeds configured maximum size (%d bytes)", maxBytes)
	}
	if !config.DocumentFormatAllowed(req.Format) {
		return fmt.Errorf("format %q is not allowed for ingest", req.Format)
	}
	if enabled, maxRunes := config.DocumentChunkingMaxChars(); enabled && maxRunes > 0 {
		if utf8.RuneCountInString(req.Content) > maxRunes {
			return fmt.Errorf("content exceeds documents.chunking.max_chunk_size (%d Unicode code points)", maxRunes)
		}
	}
	return nil
}

func (s *documentService) enqueueHotIngest(work types.HotIngestWork) {
	if s.hotIngestQueue == nil {
		s.logger.Warn("hot ingest queue not configured; running analysis inline",
			zap.String("doc_id", work.DocID))
		go func(w types.HotIngestWork) {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()
			if err := processHotIngest(ctx, w, s.docRepo, s.analyzer, s.materializer, s.tagRepo, s.logger); err != nil {
				s.logger.Error("inline hot ingest failed", zap.String("doc_id", w.DocID), zap.Error(err))
			}
		}(work)
		return
	}
	select {
	case s.hotIngestQueue <- work:
		s.logger.Info("queued hot ingest LLM work", zap.String("doc_id", work.DocID))
	default:
		s.logger.Warn("hot ingest queue full; running analysis inline",
			zap.String("doc_id", work.DocID))
		go func(w types.HotIngestWork) {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()
			if err := processHotIngest(ctx, w, s.docRepo, s.analyzer, s.materializer, s.tagRepo, s.logger); err != nil {
				s.logger.Error("fallback hot ingest failed", zap.String("doc_id", w.DocID), zap.Error(err))
			}
		}(work)
	}
}

func (s *documentService) CreateDocument(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
	if err := validateIngestRequest(req); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrIngestValidation, err)
	}

	hasPrebuiltSummary := req.Summary != "" && len(req.Chapters) > 0
	hasPrebuiltTags := len(req.Tags) > 0

	isHot := shouldBeHot(req.Content, req.EffectiveIngestMode(), hasPrebuiltSummary, s.quotaManager)

	doc := &types.Document{
		ID:         uuid.New().String(),
		Title:      req.Title,
		Content:    req.Content,
		Format:     req.Format,
		Status:     types.DocStatusCold,
		HotScore:   0,
		QueryCount: 0,
	}

	if isHot {
		doc.Status = types.DocStatusHot

		if hasPrebuiltSummary && hasPrebuiltTags {
			doc.Tags = req.Tags

			analysis := &types.DocumentAnalysisResult{
				Summary:  req.Summary,
				Tags:     req.Tags,
				Chapters: req.Chapters,
			}

			if err := s.materializer.Materialize(ctx, doc, analysis); err != nil {
				s.logger.Error("failed to materialize document with prebuilt analysis", zap.Error(err))
			}

			// Best-effort catalog tag upsert/counting.
			for _, tag := range doc.Tags {
				tagEntity := &types.Tag{ID: uuid.New().String(), Name: tag, TopicID: ""}
				if err := s.tagRepo.Create(ctx, tagEntity); err != nil {
					s.logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
				} else if err := s.tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
					s.logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
				}
			}
		} else {
			if hasPrebuiltTags {
				doc.Tags = append([]string(nil), req.Tags...)
			} else {
				doc.Tags = []string{}
			}
			doc.CreatedAt = time.Now()
			doc.UpdatedAt = doc.CreatedAt
			if err := s.docRepo.Create(ctx, doc); err != nil {
				return nil, err
			}

			work := types.HotIngestWork{DocID: doc.ID}
			if hasPrebuiltTags {
				work.PrebuiltTags = append([]string(nil), req.Tags...)
			}
			s.enqueueHotIngest(work)

			return &types.CreateDocumentResponse{
				ID:           doc.ID,
				Title:        doc.Title,
				Format:       doc.Format,
				Tags:         doc.Tags,
				Summary:      "",
				ChapterCount: 0,
				Status:       doc.Status,
				CreatedAt:    doc.CreatedAt,
			}, nil
		}
	} else {
		doc.Status = types.DocStatusCold
		doc.Tags = []string{}

		if s.coldIndex != nil {
			if err := s.coldIndex.AddDocument(ctx, doc); err != nil {
				s.logger.Warn("failed to add cold document to cold index",
					zap.String("doc_id", doc.ID),
					zap.Error(err))
			}
		}
	}

	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt
	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	return &types.CreateDocumentResponse{
		ID:           doc.ID,
		Title:        doc.Title,
		Format:       doc.Format,
		Tags:         doc.Tags,
		Summary:      "",
		ChapterCount: 0,
		Status:       doc.Status,
		CreatedAt:    doc.CreatedAt,
	}, nil
}

func (s *documentService) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return s.docRepo.GetByID(ctx, id)
}

func (s *documentService) ListRecentDocuments(ctx context.Context, limit int) ([]*types.Document, error) {
	return s.docRepo.GetRecent(ctx, limit)
}

func (s *documentService) ListDocuments(ctx context.Context) ([]types.Document, error) {
	return s.docRepo.ListAll(ctx, 1000)
}

func (s *documentService) ListHotDocumentsWithSummariesByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	return s.docRepo.ListMetaByTagsAndStatuses(ctx, tags,
		[]types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}, limit)
}

var _ service.IDocumentService = (*documentService)(nil)
