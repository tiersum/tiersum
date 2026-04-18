package catalog

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/client"
	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/markdown"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewChapterService constructs the service.IChapterService implementation.
func NewChapterService(
	chapterRepo storage.IChapterRepository,
	docRepo storage.IDocumentRepository,
	tagRepo storage.ITagRepository,
	topicRepo storage.ITopicRepository,
	coldIndex storage.IColdIndex,
	llm client.ILLMProvider,
	logger *zap.Logger,
) service.IChapterService {
	var rel *hotProgressiveLLMCore
	if llm != nil {
		rel = newHotProgressiveLLMCore(llm, logger)
	}
	return &chapterService{
		chapterRepo: chapterRepo,
		docRepo:     docRepo,
		tagRepo:     tagRepo,
		topicRepo:   topicRepo,
		coldIndex:   coldIndex,
		llm:         llm,
		logger:      logger,
		relCore:     rel,
	}
}

type chapterService struct {
	chapterRepo storage.IChapterRepository
	docRepo     storage.IDocumentRepository
	tagRepo     storage.ITagRepository
	topicRepo   storage.ITopicRepository
	coldIndex   storage.IColdIndex
	llm         client.ILLMProvider
	logger      *zap.Logger
	relCore     *hotProgressiveLLMCore
}

func (s *chapterService) ListChaptersByDocumentID(ctx context.Context, documentID string) ([]types.Chapter, error) {
	return s.chapterRepo.ListByDocument(ctx, documentID)
}

func (s *chapterService) ExtractChaptersFromMarkdown(ctx context.Context, doc *types.Document) ([]types.Chapter, error) {
	_ = ctx
	if doc == nil {
		return nil, nil
	}
	if s.coldIndex != nil {
		return s.coldIndex.MarkdownChapters(doc.ID, doc.Title, doc.Content), nil
	}
	md := strings.TrimSpace(doc.Content)
	if md == "" {
		return nil, nil
	}
	path := doc.ID + "/body"
	return []types.Chapter{{
		DocumentID: doc.ID,
		Path:       path,
		Title:      markdown.ChapterDisplayTitle(doc.ID, path, doc.Title),
		Summary:    md,
		Content:    md,
	}}, nil
}

func (s *chapterService) ListChaptersByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Chapter, error) {
	out := make(map[string][]types.Chapter, len(docIDs))
	for _, id := range docIDs {
		out[id] = nil
	}
	flat, err := s.chapterRepo.ListByDocumentIDs(ctx, docIDs)
	if err != nil {
		return nil, err
	}
	for _, ch := range flat {
		out[ch.DocumentID] = append(out[ch.DocumentID], ch)
	}
	return out, nil
}

func (s *chapterService) SearchColdChapterHits(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error) {
	if s.coldIndex == nil {
		return nil, service.ErrColdIndexUnavailable
	}
	hits, err := s.coldIndex.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]types.ColdSearchHit, len(hits))
	for i := range hits {
		out[i] = types.ColdSearchHit{
			DocumentID: hits[i].DocumentID,
			Path:       hits[i].Path,
			Title:      hits[i].Title,
			Content:    hits[i].Content,
			Score:      hits[i].Score,
			Source:     hits[i].Source,
		}
	}
	return out, nil
}

// SearchHotChapters runs the legacy progressive hot pipeline inside the chapter service:
// catalog tags (if tag count >= threshold, LLM topic pick then tag filter; else direct LLM tag filter) → documents (LLM for hot/warming, keyword for cold) → chapters (LLM), returned as ranked hits.
func (s *chapterService) SearchHotChapters(ctx context.Context, query string, limit int) ([]types.HotSearchHit, error) {
	return s.searchHotChaptersProgressive(ctx, query, limit)
}

var (
	_ service.IChapterDocumentReads    = (*chapterService)(nil)
	_ service.IChapterMarkdownFallback = (*chapterService)(nil)
	_ service.IChapterHybridSearch     = (*chapterService)(nil)
	_ service.IChapterService          = (*chapterService)(nil)
)
