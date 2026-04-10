package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage/memory"
	"github.com/tiersum/tiersum/pkg/types"
)

// parseMaxResultsFromString mirrors query param max_results parsing used by REST handlers.
func parseMaxResultsFromString(raw string, defaultVal, maxVal int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultVal
	}
	if maxVal > 0 && n > maxVal {
		return maxVal
	}
	return n
}

// ExecuteIngestDocument matches POST /api/v1/documents (after request validation).
func (h *Handler) ExecuteIngestDocument(ctx context.Context, req types.CreateDocumentRequest) (int, any) {
	doc, err := h.DocService.Ingest(ctx, req)
	if err != nil {
		h.Logger.Error("failed to ingest document", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to create document"}
	}
	return http.StatusCreated, doc
}

// ExecuteListDocuments matches GET /api/v1/documents.
func (h *Handler) ExecuteListDocuments(ctx context.Context) (int, any) {
	docs, err := h.DocService.List(ctx)
	if err != nil {
		h.Logger.Error("failed to list documents", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to list documents"}
	}
	if docs == nil {
		docs = []types.Document{}
	}
	return http.StatusOK, gin.H{"documents": docs}
}

// ExecuteGetDocument matches GET /api/v1/documents/:id.
func (h *Handler) ExecuteGetDocument(ctx context.Context, id string) (int, any) {
	doc, err := h.DocService.Get(ctx, id)
	if err != nil {
		h.Logger.Error("failed to get document", zap.String("id", id), zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to get document"}
	}
	if doc == nil {
		return http.StatusNotFound, gin.H{"error": "document not found"}
	}
	return http.StatusOK, doc
}

// ExecuteGetDocumentChapters matches GET /api/v1/documents/:id/chapters.
func (h *Handler) ExecuteGetDocumentChapters(ctx context.Context, docID string) (int, any) {
	if docID == "" {
		return http.StatusBadRequest, gin.H{"error": "document id is required"}
	}
	doc, err := h.DocService.Get(ctx, docID)
	if err != nil {
		h.Logger.Error("failed to get document", zap.String("id", docID), zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to get document"}
	}
	if doc == nil {
		return http.StatusNotFound, gin.H{"error": "document not found"}
	}
	chapters, err := h.SummaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, docID+"/")
	if err != nil {
		h.Logger.Error("failed to list chapters", zap.String("id", docID), zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to list chapters"}
	}
	out := make([]gin.H, 0, len(chapters))
	for _, s := range chapters {
		if s.IsSource {
			continue
		}
		title := strings.TrimPrefix(s.Path, docID+"/")
		out = append(out, gin.H{
			"path":    s.Path,
			"title":   title,
			"summary": s.Content,
		})
	}
	return http.StatusOK, gin.H{"document_id": docID, "chapters": out}
}

// ExecuteGetDocumentSummaries matches GET /api/v1/documents/:id/summaries.
func (h *Handler) ExecuteGetDocumentSummaries(ctx context.Context, id string) (int, any) {
	summaries, err := h.SummaryRepo.GetByDocument(ctx, id)
	if err != nil {
		h.Logger.Error("failed to get document summaries", zap.String("id", id), zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	return http.StatusOK, gin.H{"summaries": summaries}
}

// ExecuteProgressiveQuery matches POST /api/v1/query/progressive (request must satisfy binding rules).
func (h *Handler) ExecuteProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (int, any) {
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}
	response, err := h.QueryService.ProgressiveQuery(ctx, req)
	if err != nil {
		h.Logger.Error("failed to perform progressive query", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	return http.StatusOK, response
}

// ExecuteListTagGroups matches GET /api/v1/tags/groups.
func (h *Handler) ExecuteListTagGroups(ctx context.Context) (int, any) {
	if h.TagGroupService == nil {
		return http.StatusServiceUnavailable, gin.H{"error": "tag grouping service not available"}
	}
	groups, err := h.TagGroupService.GetL1Groups(ctx)
	if err != nil {
		h.Logger.Error("failed to list tag groups", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	if groups == nil {
		groups = []types.TagGroup{}
	}
	return http.StatusOK, gin.H{"groups": groups}
}

// ExecuteTriggerTagGroup matches POST /api/v1/tags/group.
func (h *Handler) ExecuteTriggerTagGroup(ctx context.Context) (int, any) {
	if h.TagGroupService == nil {
		return http.StatusServiceUnavailable, gin.H{"error": "tag grouping service not available"}
	}
	if err := h.TagGroupService.GroupTags(ctx); err != nil {
		h.Logger.Error("failed to group tags", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	return http.StatusOK, gin.H{"message": "tag grouping triggered"}
}

// ExecuteListTags matches GET /api/v1/tags (group_ids optional; max_results optional string like the query param).
func (h *Handler) ExecuteListTags(ctx context.Context, groupIDs []string, maxResultsQuery string) (int, any) {
	var tags []types.Tag
	var err error
	if len(groupIDs) > 0 {
		n := parseMaxResultsFromString(maxResultsQuery, 100, 10000)
		tags, err = h.TagRepo.ListByGroupIDs(ctx, groupIDs, n)
	} else {
		tags, err = h.TagRepo.List(ctx)
		if err == nil {
			if raw := strings.TrimSpace(maxResultsQuery); raw != "" {
				if capN, e := strconv.Atoi(raw); e == nil && capN > 0 {
					if capN > 10000 {
						capN = 10000
					}
					if len(tags) > capN {
						tags = tags[:capN]
					}
				}
			}
		}
	}
	if err != nil {
		h.Logger.Error("failed to list tags", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	if tags == nil {
		tags = []types.Tag{}
	}
	return http.StatusOK, gin.H{"tags": tags}
}

// ExecuteHotDocSummaries matches GET /api/v1/hot/doc_summaries.
func (h *Handler) ExecuteHotDocSummaries(ctx context.Context, tags []string, maxResultsQuery string) (int, any) {
	if h.DocRepo == nil {
		return http.StatusServiceUnavailable, gin.H{"error": "document repository not configured"}
	}
	if len(tags) == 0 {
		return http.StatusBadRequest, gin.H{"error": "tags query parameter is required (comma-separated)"}
	}
	n := parseMaxResultsFromString(maxResultsQuery, 1000, 10000)
	docs, err := h.DocRepo.ListMetaByTagsAndStatuses(ctx, tags,
		[]types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}, n)
	if err != nil {
		h.Logger.Error("hot doc_summaries", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	if len(docs) == 0 {
		return http.StatusOK, gin.H{"items": []any{}}
	}
	ids := make([]string, len(docs))
	for i := range docs {
		ids[i] = docs[i].ID
	}
	sumRows, err := h.SummaryRepo.ListDocumentTierByDocumentIDs(ctx, ids)
	if err != nil {
		h.Logger.Error("hot doc_summaries summaries", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	byDoc := make(map[string]string, len(sumRows))
	for _, s := range sumRows {
		byDoc[s.DocumentID] = s.Content
	}
	items := make([]gin.H, 0, len(docs))
	for _, d := range docs {
		items = append(items, gin.H{
			"document_id": d.ID,
			"title":       d.Title,
			"format":      d.Format,
			"status":      d.Status,
			"tags":        d.Tags,
			"summary":     byDoc[d.ID],
		})
	}
	return http.StatusOK, gin.H{"items": items}
}

// ExecuteHotDocChapters matches GET /api/v1/hot/doc_chapters.
func (h *Handler) ExecuteHotDocChapters(ctx context.Context, docIDs []string, maxResultsQuery string) (int, any) {
	if len(docIDs) == 0 {
		return http.StatusBadRequest, gin.H{"error": "doc_ids query parameter is required (comma-separated)"}
	}
	maxDocs := parseMaxResultsFromString(maxResultsQuery, 100, 500)
	if len(docIDs) > maxDocs {
		docIDs = docIDs[:maxDocs]
	}
	docsOut := make([]gin.H, 0, len(docIDs))
	for _, docID := range docIDs {
		chapters, err := h.SummaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, docID+"/")
		if err != nil {
			h.Logger.Error("hot doc_chapters", zap.String("doc_id", docID), zap.Error(err))
			return http.StatusInternalServerError, gin.H{"error": err.Error()}
		}
		chOut := make([]gin.H, 0, len(chapters))
		for _, s := range chapters {
			if s.IsSource {
				continue
			}
			title := strings.TrimPrefix(s.Path, docID+"/")
			chOut = append(chOut, gin.H{
				"path":    s.Path,
				"title":   title,
				"summary": s.Content,
			})
		}
		docsOut = append(docsOut, gin.H{
			"document_id": docID,
			"chapters":    chOut,
		})
	}
	return http.StatusOK, gin.H{"documents": docsOut}
}

// ExecuteHotDocSource matches GET /api/v1/hot/doc_source.
func (h *Handler) ExecuteHotDocSource(ctx context.Context, paths []string, maxResultsQuery string) (int, any) {
	if len(paths) == 0 {
		return http.StatusBadRequest, gin.H{"error": "chapter_paths query parameter is required (comma-separated)"}
	}
	capN := parseMaxResultsFromString(maxResultsQuery, 100, 2000)
	summaries, err := h.SummaryRepo.ListSourcesByPaths(ctx, paths)
	if err != nil {
		h.Logger.Error("hot doc_source", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	if capN > 0 && len(summaries) > capN {
		summaries = summaries[:capN]
	}
	items := make([]gin.H, 0, len(summaries))
	for _, s := range summaries {
		items = append(items, gin.H{
			"chapter_path": strings.TrimSuffix(s.Path, "/source"),
			"path":         s.Path,
			"document_id":  s.DocumentID,
			"content":      s.Content,
		})
	}
	return http.StatusOK, gin.H{"items": items}
}

// ExecuteColdDocSource matches GET /api/v1/cold/doc_source.
func (h *Handler) ExecuteColdDocSource(ctx context.Context, terms []string, maxResultsQuery string) (int, any) {
	if h.MemIndex == nil {
		return http.StatusServiceUnavailable, gin.H{"error": "cold document index not available"}
	}
	if len(terms) == 0 {
		return http.StatusBadRequest, gin.H{"error": "q query parameter is required (comma-separated keywords)"}
	}
	n := parseMaxResultsFromString(maxResultsQuery, 100, 500)
	queryText := strings.Join(terms, " ")
	emb := memory.GenerateSimpleEmbedding(queryText)
	results, err := h.MemIndex.HybridSearch(queryText, emb, n)
	if err != nil {
		h.Logger.Error("cold doc_source", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	items := make([]gin.H, 0, len(results))
	for _, r := range results {
		entry := gin.H{
			"document_id": r.DocumentID,
			"title":       r.Title,
			"score":       r.Score,
			"source":      r.Source,
			"context":     r.Content,
		}
		if len(r.Snippets) > 0 {
			snips := make([]gin.H, 0, len(r.Snippets))
			for _, sn := range r.Snippets {
				snips = append(snips, gin.H{
					"text":      sn.Text,
					"start_pos": sn.StartPos,
					"end_pos":   sn.EndPos,
					"keyword":   sn.Keyword,
				})
			}
			entry["snippets"] = snips
		}
		items = append(items, entry)
	}
	return http.StatusOK, gin.H{"items": items}
}

// ExecuteGetQuota matches GET /api/v1/quota.
func (h *Handler) ExecuteGetQuota() (int, any) {
	if h.Quota == nil {
		return http.StatusOK, gin.H{
			"used":     0,
			"total":    100,
			"reset_at": nil,
		}
	}
	used, total, resetAt := h.Quota.GetQuota()
	return http.StatusOK, gin.H{
		"used":     used,
		"total":    total,
		"reset_at": resetAt,
	}
}
