package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage/memory"
	"github.com/tiersum/tiersum/pkg/types"
)

// maxResultsQueryParam is the unified query-string name for capping result counts on list/search endpoints.
const maxResultsQueryParam = "max_results"

func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseQueryMaxResults(c *gin.Context, defaultVal, maxVal int) int {
	raw := strings.TrimSpace(c.Query(maxResultsQueryParam))
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

// ListTags handles GET /tags with optional group_ids and max_results.
func (h *Handler) ListTags(c *gin.Context) {
	ctx := c.Request.Context()
	groupIDs := parseCommaSeparated(c.Query("group_ids"))

	var tags []types.Tag
	var err error
	if len(groupIDs) > 0 {
		n := parseQueryMaxResults(c, 100, 10000)
		tags, err = h.TagRepo.ListByGroupIDs(ctx, groupIDs, n)
	} else {
		tags, err = h.TagRepo.List(ctx)
		if err == nil {
			if raw := strings.TrimSpace(c.Query(maxResultsQueryParam)); raw != "" {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tags == nil {
		tags = []types.Tag{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// HotDocSummaries handles GET /hot/doc_summaries — hot/warming docs matching tags; document-level summary only (no body).
func (h *Handler) HotDocSummaries(c *gin.Context) {
	if h.DocRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "document repository not configured"})
		return
	}
	tags := parseCommaSeparated(c.Query("tags"))
	if len(tags) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tags query parameter is required (comma-separated)"})
		return
	}
	n := parseQueryMaxResults(c, 1000, 10000)
	ctx := c.Request.Context()

	docs, err := h.DocRepo.ListMetaByTagsAndStatuses(ctx, tags,
		[]types.DocumentStatus{types.DocStatusHot, types.DocStatusWarming}, n)
	if err != nil {
		h.Logger.Error("hot doc_summaries", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(docs) == 0 {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}
	ids := make([]string, len(docs))
	for i := range docs {
		ids[i] = docs[i].ID
	}
	sumRows, err := h.SummaryRepo.ListDocumentTierByDocumentIDs(ctx, ids)
	if err != nil {
		h.Logger.Error("hot doc_summaries summaries", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// HotDocChapters handles GET /hot/doc_chapters — chapter-level summaries (not source) per document.
func (h *Handler) HotDocChapters(c *gin.Context) {
	docIDs := parseCommaSeparated(c.Query("doc_ids"))
	if len(docIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "doc_ids query parameter is required (comma-separated)"})
		return
	}
	maxDocs := parseQueryMaxResults(c, 100, 500)
	if len(docIDs) > maxDocs {
		docIDs = docIDs[:maxDocs]
	}
	ctx := c.Request.Context()
	docsOut := make([]gin.H, 0, len(docIDs))

	for _, docID := range docIDs {
		chapters, err := h.SummaryRepo.QueryByTierAndPrefix(ctx, types.TierChapter, docID+"/")
		if err != nil {
			h.Logger.Error("hot doc_chapters", zap.String("doc_id", docID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
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
	c.JSON(http.StatusOK, gin.H{"documents": docsOut})
}

// HotDocSource handles GET /hot/doc_source — original chapter text for given chapter paths.
func (h *Handler) HotDocSource(c *gin.Context) {
	paths := parseCommaSeparated(c.Query("chapter_paths"))
	if len(paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chapter_paths query parameter is required (comma-separated)"})
		return
	}
	capN := parseQueryMaxResults(c, 100, 2000)
	ctx := c.Request.Context()
	summaries, err := h.SummaryRepo.ListSourcesByPaths(ctx, paths)
	if err != nil {
		h.Logger.Error("hot doc_source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// ColdDocSource handles GET /cold/doc_source — locate query terms in cold docs via memory index; returns context snippets (no tag filter).
func (h *Handler) ColdDocSource(c *gin.Context) {
	if h.MemIndex == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cold document index not available"})
		return
	}
	terms := parseCommaSeparated(c.Query("q"))
	if len(terms) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q query parameter is required (comma-separated keywords)"})
		return
	}
	n := parseQueryMaxResults(c, 100, 500)
	queryText := strings.Join(terms, " ")
	emb := memory.GenerateSimpleEmbedding(queryText)
	results, err := h.MemIndex.HybridSearch(queryText, emb, n)
	if err != nil {
		h.Logger.Error("cold doc_source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}
