package document

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

func NewDocumentService(
	docRepo storage.IDocumentRepository,
	cold storage.IColdIndex,
	tagRepo storage.ITagRepository,
	chapterRepo storage.IChapterRepository,
	hotIngestSink service.IHotIngestWorkSink,
	logger *zap.Logger,
) service.IDocumentService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &documentService{
		docs:          docRepo,
		cold:          cold,
		tags:          tagRepo,
		chapters:      chapterRepo,
		hotIngestSink: hotIngestSink,
		logger:        logger,
	}
}

type documentService struct {
	docs          storage.IDocumentRepository
	cold          storage.IColdIndex
	tags          storage.ITagRepository
	chapters      storage.IChapterRepository
	hotIngestSink service.IHotIngestWorkSink
	logger        *zap.Logger
}

func (s *documentService) CreateDocument(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/document")
	ctx, span := tr.Start(ctx, "CreateDocument", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("title", req.Title))
	span.SetAttributes(attribute.String("ingest_mode", req.IngestMode))
	span.SetAttributes(attribute.Bool("force_hot", req.ForceHot))
	span.SetAttributes(attribute.Int("content_len", len(req.Content)))

	if s.docs == nil {
		return nil, errors.New("document repository not configured")
	}
	if err := validateCreateIngest(req); err != nil {
		return nil, err
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		return nil, fmt.Errorf("%w: format is required", service.ErrIngestValidation)
	}

	hot := resolveHotIngest(req)
	tags := dedupeTagNames(req.Tags)
	if !hot {
		tags = nil
	}

	summary := strings.TrimSpace(req.Summary)
	docID := strings.TrimSpace(uuid.New().String())
	doc := &types.Document{
		ID:         docID,
		Title:      strings.TrimSpace(req.Title),
		Summary:    summary,
		Content:    req.Content,
		Format:     format,
		Tags:       tags,
		Status:     types.DocStatusCold,
		HotScore:   0,
		QueryCount: 0,
	}
	if hot {
		doc.Status = types.DocStatusHot
	}

	chapterCount := 0
	if hot {
		if err := s.docs.Create(ctx, doc); err != nil {
			return nil, fmt.Errorf("persist document: %w", err)
		}
		if len(req.Chapters) > 0 && s.chapters != nil {
			chRows := materializePrebuiltChapters(doc.ID, req.Chapters)
			if err := s.chapters.ReplaceByDocument(ctx, doc.ID, chRows); err != nil {
				return nil, fmt.Errorf("persist prebuilt chapters: %w", err)
			}
			chapterCount = len(chRows)
		}
		if err := s.syncCatalogTags(ctx, tags); err != nil {
			return nil, err
		}
	} else if s.cold != nil {
		if err := s.docs.Create(ctx, doc); err != nil {
			return nil, fmt.Errorf("persist document: %w", err)
		}
		if s.chapters != nil {
			coldChapters := s.cold.MarkdownChapters(doc.ID, doc.Title, doc.Content)
			if len(coldChapters) > 0 {
				if err := s.chapters.ReplaceByDocument(ctx, doc.ID, coldChapters); err != nil {
					s.logger.Error("failed to persist cold chapters",
						zap.String("doc_id", doc.ID),
						zap.Error(err))
				} else {
					chapterCount = len(coldChapters)
				}
			}
		}
	} else {
		if err := s.docs.Create(ctx, doc); err != nil {
			return nil, fmt.Errorf("persist document: %w", err)
		}
	}

	stored, err := s.docs.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("reload document: %w", err)
	}
	if stored == nil {
		return nil, errors.New("document missing after create")
	}
	if hot && len(req.Chapters) == 0 && s.hotIngestSink != nil {
		s.hotIngestSink.SubmitHotIngest(types.HotIngestWork{DocID: doc.ID, PrebuiltTags: tags})
	}
	if chapterCount == 0 && hot && s.chapters != nil {
		if list, err := s.chapters.ListByDocument(ctx, doc.ID); err == nil {
			chapterCount = len(list)
		}
	}

	return &types.CreateDocumentResponse{
		ID:           stored.ID,
		Title:        stored.Title,
		Format:       stored.Format,
		Tags:         stored.Tags,
		Summary:      stored.Summary,
		ChapterCount: chapterCount,
		Status:       stored.Status,
		CreatedAt:    stored.CreatedAt,
	}, nil
}

func validateCreateIngest(req types.CreateDocumentRequest) error {
	if int64(len(req.Content)) > config.DocumentMaxBodyBytes() {
		return fmt.Errorf("%w: content exceeds documents.max_size", service.ErrIngestValidation)
	}
	if !config.DocumentFormatAllowed(req.Format) {
		return fmt.Errorf("%w: format not allowed by documents.supported_formats", service.ErrIngestValidation)
	}
	if enabled, maxR := config.DocumentChunkingMaxChars(); enabled && maxR > 0 {
		if utf8.RuneCountInString(req.Content) > maxR {
			return fmt.Errorf("%w: content exceeds documents.chunking.max_chunk_size (runes)", service.ErrIngestValidation)
		}
	}
	return nil
}

func resolveHotIngest(req types.CreateDocumentRequest) bool {
	return req.EffectiveIngestMode() == types.DocumentIngestModeHot
}

func mergeOrderedTagLists(first, second []string) []string {
	seen := make(map[string]struct{}, len(first)+len(second))
	out := make([]string, 0, len(first)+len(second))
	push := func(raw string) {
		t := strings.TrimSpace(raw)
		if t == "" {
			return
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	for _, raw := range first {
		push(raw)
	}
	for _, raw := range second {
		push(raw)
	}
	return out
}

func dedupeTagNames(in []string) []string {
	return mergeOrderedTagLists(in, nil)
}

func materializePrebuiltChapters(docID string, infos []types.ChapterInfo) []types.Chapter {
	out := make([]types.Chapter, 0, len(infos))
	for i, ci := range infos {
		title := strings.TrimSpace(ci.Title)
		if title == "" {
			title = fmt.Sprintf("Section %d", i+1)
		}
		path := chapterPathForPrebuilt(docID, i, title)
		content := ci.Content
		summ := strings.TrimSpace(ci.Summary)
		if summ == "" && content != "" {
			summ = strings.TrimSpace(content)
		}
		if content == "" && summ != "" {
			content = summ
		}
		out = append(out, types.Chapter{
			DocumentID: docID,
			Path:       path,
			Title:      title,
			Summary:    summ,
			Content:    content,
		})
	}
	return out
}

func chapterPathForPrebuilt(docID string, idx int, _ string) string {
	return fmt.Sprintf("%s/__ingest/%d", docID, idx+1)
}

func (s *documentService) syncCatalogTags(ctx context.Context, tagNames []string) error {
	if s.tags == nil || len(tagNames) == 0 {
		return nil
	}
	for _, name := range tagNames {
		existing, err := s.tags.GetByName(ctx, name)
		if err != nil {
			return fmt.Errorf("catalog tag lookup %q: %w", name, err)
		}
		if existing == nil {
			if err := s.tags.Create(ctx, &types.Tag{Name: name, DocumentCount: 0}); err != nil {
				return fmt.Errorf("catalog tag create %q: %w", name, err)
			}
		}
		if err := s.tags.IncrementDocumentCount(ctx, name); err != nil {
			return fmt.Errorf("catalog tag increment %q: %w", name, err)
		}
	}
	return nil
}

func (s *documentService) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/document")
	ctx, span := tr.Start(ctx, "GetDocument", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("doc_id", id))

	if s.docs == nil {
		return nil, errors.New("document repository not configured")
	}
	return s.docs.GetByID(ctx, id)
}

func (s *documentService) CountDocumentsByStatus(ctx context.Context) (types.DocumentStatusCounts, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/document")
	ctx, span := tr.Start(ctx, "CountDocumentsByStatus", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	if s.docs == nil {
		return types.DocumentStatusCounts{}, errors.New("document repository not configured")
	}
	return s.docs.CountDocumentsByStatus(ctx)
}

func (s *documentService) ListDocuments(ctx context.Context, limit int) ([]types.Document, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/document")
	ctx, span := tr.Start(ctx, "ListDocuments", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.Int("limit", limit))

	if limit <= 0 {
		limit = 200
	}
	if s.docs == nil {
		return nil, errors.New("document repository not configured")
	}
	rows, err := s.docs.GetRecent(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]types.Document, 0, len(rows))
	for _, p := range rows {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (s *documentService) ListHotDocumentsWithSummariesByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error) {
	tr := otel.Tracer("github.com/tiersum/tiersum/service/document")
	ctx, span := tr.Start(ctx, "ListHotDocumentsWithSummariesByTags", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.Int("tag_count", len(tags)))
	span.SetAttributes(attribute.Int("limit", limit))

	if s.docs == nil {
		return nil, errors.New("document repository not configured")
	}
	tags = dedupeTagNames(tags)
	if len(tags) == 0 {
		return []types.Document{}, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	statuses := []types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}
	return s.docs.ListMetaByTagsAndStatuses(ctx, tags, statuses, limit)
}

var _ service.IDocumentService = (*documentService)(nil)
