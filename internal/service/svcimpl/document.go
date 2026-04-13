package svcimpl

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

// quotaGate abstracts hourly hot-ingest quota (real *QuotaManager or test doubles).
type quotaGate interface {
	CheckAndConsume() bool
}

// DocumentSvc implements service.IDocumentService
type DocumentSvc struct {
	docRepo        storage.IDocumentRepository
	indexer        service.IIndexer
	summarizer     service.ISummarizer
	tagRepo        storage.ITagRepository
	coldIndex      storage.IColdIndex
	quotaManager   quotaGate
	logger         *zap.Logger
	hotIngestQueue chan<- types.HotIngestWork
}

// NewDocumentSvc creates a new document service.
// hotIngestQueue receives work when a hot document needs deferred LLM analysis; pass job.HotIngestQueue from the composition root.
func NewDocumentSvc(
	docRepo storage.IDocumentRepository,
	indexer service.IIndexer,
	summarizer service.ISummarizer,
	tagRepo storage.ITagRepository,
	coldIndex storage.IColdIndex,
	quotaManager quotaGate,
	logger *zap.Logger,
	hotIngestQueue chan<- types.HotIngestWork,
) *DocumentSvc {
	return &DocumentSvc{
		docRepo:        docRepo,
		indexer:        indexer,
		summarizer:     summarizer,
		tagRepo:        tagRepo,
		coldIndex:      coldIndex,
		quotaManager:   quotaManager,
		logger:         logger,
		hotIngestQueue: hotIngestQueue,
	}
}

// shouldBeHot determines if a document should use the hot ingest path.
// mode is from CreateDocumentRequest.EffectiveIngestMode (auto | hot | cold).
func (s *DocumentSvc) shouldBeHot(content string, mode string, hasPrebuiltSummary bool) bool {
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
		if !s.quotaManager.CheckAndConsume() {
			return false
		}
		return len(content) > config.HotContentThreshold()
	}
}

func (s *DocumentSvc) enqueueHotIngest(work types.HotIngestWork) {
	if s.hotIngestQueue == nil {
		s.logger.Warn("hot ingest queue not configured; running analysis inline",
			zap.String("doc_id", work.DocID))
		go func(w types.HotIngestWork) {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()
			if err := s.ProcessHotIngestWork(ctx, w); err != nil {
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
			if err := s.ProcessHotIngestWork(ctx, w); err != nil {
				s.logger.Error("fallback hot ingest failed", zap.String("doc_id", w.DocID), zap.Error(err))
			}
		}(work)
	}
}

// ProcessHotIngestWork runs LLM analysis, updates tags, indexes summaries, and updates global tags.
func (s *DocumentSvc) ProcessHotIngestWork(ctx context.Context, work types.HotIngestWork) error {
	if work.DocID == "" {
		return nil
	}
	doc, err := s.docRepo.GetByID(ctx, work.DocID)
	if err != nil {
		return err
	}
	if doc == nil {
		s.logger.Warn("hot ingest: document not found", zap.String("doc_id", work.DocID))
		return nil
	}
	if doc.Status != types.DocStatusHot {
		s.logger.Info("hot ingest: skipping non-hot document",
			zap.String("doc_id", work.DocID),
			zap.String("status", string(doc.Status)))
		return nil
	}

	prebuilt := work.PrebuiltTags
	analysis, err := s.summarizer.AnalyzeDocument(ctx, doc.Title, doc.Content)
	if err != nil {
		s.logger.Warn("failed to analyze document (async)", zap.String("doc_id", doc.ID), zap.Error(err))
		if len(prebuilt) > 0 {
			analysis = &types.DocumentAnalysisResult{
				Summary:  truncateContent(doc.Content, 200),
				Tags:     append([]string(nil), prebuilt...),
				Chapters: []types.ChapterInfo{},
			}
		} else {
			analysis = &types.DocumentAnalysisResult{
				Summary:  truncateContent(doc.Content, 200),
				Tags:     []string{},
				Chapters: []types.ChapterInfo{},
			}
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

	if err := s.docRepo.UpdateTags(ctx, doc.ID, mergedTags); err != nil {
		return fmt.Errorf("update tags after async analysis: %w", err)
	}
	doc.Tags = mergedTags

	if err := s.indexer.Index(ctx, doc, analysis); err != nil {
		s.logger.Error("failed to index document (async)", zap.String("doc_id", doc.ID), zap.Error(err))
		return err
	}

	for _, tag := range doc.Tags {
		tagEntity := &types.Tag{
			ID:      uuid.New().String(),
			Name:    tag,
			TopicID: "",
		}
		if err := s.tagRepo.Create(ctx, tagEntity); err != nil {
			s.logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
		} else {
			if err := s.tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
				s.logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
			}
		}
	}
	return nil
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

// Ingest implements IDocumentService.Ingest
func (s *DocumentSvc) Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
	if err := validateIngestRequest(req); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrIngestValidation, err)
	}

	// Check if document has prebuilt summary/tags from external agent
	hasPrebuiltSummary := req.Summary != "" && len(req.Chapters) > 0
	hasPrebuiltTags := len(req.Tags) > 0

	// Determine if document should be hot or cold
	isHot := s.shouldBeHot(req.Content, req.EffectiveIngestMode(), hasPrebuiltSummary)

	// Create document entity
	doc := &types.Document{
		ID:         uuid.New().String(),
		Title:      req.Title,
		Content:    req.Content,
		Format:     req.Format,
		Status:     types.DocStatusCold, // Default to cold
		HotScore:   0,
		QueryCount: 0,
	}

	if isHot {
		doc.Status = types.DocStatusHot

		if hasPrebuiltSummary && hasPrebuiltTags {
			// External agent provided everything - no internal LLM needed
			doc.Tags = req.Tags

			analysis := &types.DocumentAnalysisResult{
				Summary:  req.Summary,
				Tags:     req.Tags,
				Chapters: req.Chapters,
			}

			if err := s.indexer.Index(ctx, doc, analysis); err != nil {
				s.logger.Error("failed to index document with prebuilt analysis", zap.Error(err))
			}

			for _, tag := range doc.Tags {
				tagEntity := &types.Tag{
					ID:      uuid.New().String(),
					Name:    tag,
					TopicID: "",
				}
				if err := s.tagRepo.Create(ctx, tagEntity); err != nil {
					s.logger.Warn("failed to create global tag", zap.String("tag", tag), zap.Error(err))
				} else {
					if err := s.tagRepo.IncrementDocumentCount(ctx, tag); err != nil {
						s.logger.Warn("failed to increment tag count", zap.String("tag", tag), zap.Error(err))
					}
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

			s.logger.Info("document ingested",
				zap.String("id", doc.ID),
				zap.String("title", doc.Title),
				zap.String("status", string(doc.Status)),
				zap.Int("tags", len(doc.Tags)),
				zap.Bool("hot_ingest_async", true))

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
		// Cold document: minimal processing, no tags, no LLM
		doc.Status = types.DocStatusCold
		doc.Tags = []string{}

		if s.coldIndex != nil {
			if err := s.coldIndex.AddDocument(ctx, doc); err != nil {
				s.logger.Warn("failed to add cold document to cold index",
					zap.String("doc_id", doc.ID),
					zap.Error(err))
			}
		}

		s.logger.Info("cold document ingested",
			zap.String("id", doc.ID),
			zap.Int("content_length", len(doc.Content)))
	}

	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt
	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	s.logger.Info("document ingested",
		zap.String("id", doc.ID),
		zap.String("title", doc.Title),
		zap.String("status", string(doc.Status)),
		zap.Int("tags", len(doc.Tags)))

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

// Get implements IDocumentService.Get
func (s *DocumentSvc) Get(ctx context.Context, id string) (*types.Document, error) {
	return s.docRepo.GetByID(ctx, id)
}

// GetRecent implements IDocumentService.GetRecent
func (s *DocumentSvc) GetRecent(ctx context.Context, limit int) ([]*types.Document, error) {
	return s.docRepo.GetRecent(ctx, limit)
}

// List implements IDocumentService.List
func (s *DocumentSvc) List(ctx context.Context) ([]types.Document, error) {
	return s.docRepo.ListAll(ctx, 1000)
}

// truncateContent truncates content to max length
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

var _ service.IDocumentService = (*DocumentSvc)(nil)
var _ service.IHotIngestProcessor = (*DocumentSvc)(nil)
