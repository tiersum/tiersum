package svcimpl

import (
	"context"
	"fmt"
	"strings"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/coldindex"
	"github.com/tiersum/tiersum/pkg/types"
)

// RetrievalSvc implements service.IRetrievalService for REST/MCP read paths only.
type RetrievalSvc struct {
	tagRepo     storage.ITagRepository
	summaryRepo storage.ISummaryRepository
	docRepo     storage.IDocumentRepository
	coldIndex   storage.IColdIndex
}

// NewRetrievalSvc wires read-only API operations over storage.
func NewRetrievalSvc(
	tagRepo storage.ITagRepository,
	summaryRepo storage.ISummaryRepository,
	docRepo storage.IDocumentRepository,
	coldIndex storage.IColdIndex,
) *RetrievalSvc {
	return &RetrievalSvc{
		tagRepo:     tagRepo,
		summaryRepo: summaryRepo,
		docRepo:     docRepo,
		coldIndex:   coldIndex,
	}
}

// ListTags returns catalog tags, optionally filtered by topic IDs with byTopicLimit, or all tags capped by listAllCap (0 = no cap).
func (s *RetrievalSvc) ListTags(ctx context.Context, topicIDs []string, byTopicLimit int, listAllCap int) ([]types.Tag, error) {
	if len(topicIDs) > 0 {
		if byTopicLimit <= 0 {
			byTopicLimit = 100
		}
		return s.tagRepo.ListByTopicIDs(ctx, topicIDs, byTopicLimit)
	}
	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	if listAllCap > 0 && len(tags) > listAllCap {
		tags = tags[:listAllCap]
	}
	return tags, nil
}

// ListSummariesForDocument returns all summary rows for a document.
func (s *RetrievalSvc) ListSummariesForDocument(ctx context.Context, documentID string) ([]types.Summary, error) {
	return s.summaryRepo.GetByDocument(ctx, documentID)
}

// ListChapterSummariesForDocument returns chapter-tier summaries for a document (including source rows; callers filter as needed).
func (s *RetrievalSvc) ListChapterSummariesForDocument(ctx context.Context, documentID string) ([]types.Summary, error) {
	return s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, documentID+"/")
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

// MarkdownChaptersForDocument implements service.IRetrievalService.
func (s *RetrievalSvc) MarkdownChaptersForDocument(ctx context.Context, doc *types.Document) ([]types.DocumentMarkdownChapter, error) {
	_ = ctx
	if doc == nil {
		return nil, nil
	}
	if s.coldIndex != nil {
		return s.coldIndex.MarkdownChapters(doc.ID, doc.Title, doc.Content), nil
	}
	parts := coldindex.SplitMarkdown(doc.ID, doc.Title, doc.Content, types.DefaultColdChapterMaxTokens)
	out := make([]types.DocumentMarkdownChapter, 0, len(parts))
	for _, p := range parts {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		out = append(out, types.DocumentMarkdownChapter{
			Path:    p.Path,
			Title:   markdownChapterTitle(doc.ID, p.Path, doc.Title),
			Content: text,
		})
	}
	if len(out) == 0 {
		md := strings.TrimSpace(doc.Content)
		if md == "" {
			return nil, nil
		}
		path := doc.ID + "/body"
		return []types.DocumentMarkdownChapter{{
			Path:    path,
			Title:   markdownChapterTitle(doc.ID, path, doc.Title),
			Content: md,
		}}, nil
	}
	return out, nil
}

// HotDocumentsWithDocSummaries returns hot/warming document metadata and matching document-tier summaries.
func (s *RetrievalSvc) HotDocumentsWithDocSummaries(ctx context.Context, tags []string, limit int) ([]types.Document, []types.Summary, error) {
	docs, err := s.docRepo.ListMetaByTagsAndStatuses(ctx, tags,
		[]types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}, limit)
	if err != nil || len(docs) == 0 {
		return docs, nil, err
	}
	ids := make([]string, len(docs))
	for i := range docs {
		ids[i] = docs[i].ID
	}
	sums, err := s.summaryRepo.ListDocumentTierByDocumentIDs(ctx, ids)
	return docs, sums, err
}

// ChapterSummariesByDocumentIDs returns chapter-tier summaries per document ID (query order preserved).
func (s *RetrievalSvc) ChapterSummariesByDocumentIDs(ctx context.Context, docIDs []string) (map[string][]types.Summary, error) {
	out := make(map[string][]types.Summary, len(docIDs))
	for _, id := range docIDs {
		ch, err := s.summaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, id+"/")
		if err != nil {
			return nil, fmt.Errorf("chapters for %s: %w", id, err)
		}
		out[id] = ch
	}
	return out, nil
}

// ListSourcesByChapterPaths returns source-tier rows for the given paths.
func (s *RetrievalSvc) ListSourcesByChapterPaths(ctx context.Context, paths []string) ([]types.Summary, error) {
	return s.summaryRepo.ListSourcesByPaths(ctx, paths)
}

// SearchColdByQuery runs hybrid cold search and maps hits to API types.
func (s *RetrievalSvc) SearchColdByQuery(ctx context.Context, query string, limit int) ([]types.ColdSearchHit, error) {
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

// ApproxColdIndexEntries implements service.IRetrievalService.
func (s *RetrievalSvc) ApproxColdIndexEntries() int {
	if s.coldIndex == nil {
		return 0
	}
	return s.coldIndex.ApproxEntries()
}

// ColdIndexVectorStats implements service.IRetrievalService.
func (s *RetrievalSvc) ColdIndexVectorStats() storage.ColdIndexVectorStats {
	if s.coldIndex == nil {
		return storage.ColdIndexVectorStats{}
	}
	return s.coldIndex.VectorStats()
}

// ColdIndexInvertedStats implements service.IRetrievalService.
func (s *RetrievalSvc) ColdIndexInvertedStats() storage.ColdIndexInvertedStats {
	if s.coldIndex == nil {
		return storage.ColdIndexInvertedStats{}
	}
	return s.coldIndex.InvertedIndexStats()
}

var _ service.IRetrievalService = (*RetrievalSvc)(nil)
