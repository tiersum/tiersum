package catalog

import (
	"context"
	"strings"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/coldindex"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewChapterService constructs the IChapterService implementation.
func NewChapterService(chapterRepo storage.IChapterRepository, coldIndex storage.IColdIndex) service.IChapterService {
	return &chapterService{chapterRepo: chapterRepo, coldIndex: coldIndex}
}

type chapterService struct {
	chapterRepo storage.IChapterRepository
	coldIndex   storage.IColdIndex
}

func (s *chapterService) ListChaptersByDocumentID(ctx context.Context, documentID string) ([]types.Chapter, error) {
	return s.chapterRepo.ListByDocument(ctx, documentID)
}

func markdownChapterTitle(docID, path, fallback string) string {
	rel := strings.TrimPrefix(path, docID+"/")
	if rel == "" {
		if strings.TrimSpace(fallback) != "" {
			return fallback
		}
		return "Document"
	}
	return strings.ReplaceAll(rel, "/", " · ")
}

func (s *chapterService) ExtractChaptersFromMarkdown(ctx context.Context, doc *types.Document) ([]types.Chapter, error) {
	_ = ctx
	if doc == nil {
		return nil, nil
	}
	if s.coldIndex != nil {
		return s.coldIndex.MarkdownChapters(doc.ID, doc.Title, doc.Content), nil
	}
	parts := coldindex.SplitMarkdown(doc.ID, doc.Title, doc.Content, types.DefaultColdChapterMaxTokens)
	out := make([]types.Chapter, 0, len(parts))
	for _, p := range parts {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		out = append(out, types.Chapter{
			DocumentID: doc.ID,
			Path:       p.Path,
			Title:      markdownChapterTitle(doc.ID, p.Path, doc.Title),
			Summary:    text,
			Content:    text,
		})
	}
	if len(out) == 0 {
		md := strings.TrimSpace(doc.Content)
		if md == "" {
			return nil, nil
		}
		path := doc.ID + "/body"
		return []types.Chapter{{
			DocumentID: doc.ID,
			Path:       path,
			Title:      markdownChapterTitle(doc.ID, path, doc.Title),
			Summary:    md,
			Content:    md,
		}}, nil
	}
	return out, nil
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

// SearchColdChapterHits runs hybrid cold search and maps storage.ColdIndexHit rows to API DTOs.
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

var _ service.IChapterService = (*chapterService)(nil)
