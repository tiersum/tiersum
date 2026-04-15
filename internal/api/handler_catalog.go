package api

import (
	"strings"

	"github.com/gin-gonic/gin"
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

// ListTags handles GET /tags with optional topic_ids and max_results.
func (h *Handler) ListTags(c *gin.Context) {
	ctx := c.Request.Context()
	topicIDs := parseCommaSeparated(c.Query("topic_ids"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteListTags(ctx, topicIDs, maxRaw)
	c.JSON(status, body)
}

// ListHotDocumentSummariesByTags handles GET /hot/doc_summaries — hot/warming docs matching tags; document-level summary only (no body).
func (h *Handler) ListHotDocumentSummariesByTags(c *gin.Context) {
	tags := parseCommaSeparated(c.Query("tags"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteListHotDocumentSummariesByTags(c.Request.Context(), tags, maxRaw)
	c.JSON(status, body)
}

// ListHotDocumentChaptersByDocumentIDs handles GET /hot/doc_chapters — chapter-level summaries (not source) per document.
func (h *Handler) ListHotDocumentChaptersByDocumentIDs(c *gin.Context) {
	docIDs := parseCommaSeparated(c.Query("doc_ids"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteListHotDocumentChaptersByDocumentIDs(c.Request.Context(), docIDs, maxRaw)
	c.JSON(status, body)
}

// SearchColdChapterHits handles GET /cold/doc_source — hybrid search over cold document chapters; returns full chapter text and optional path (no tag filter).
func (h *Handler) SearchColdChapterHits(c *gin.Context) {
	terms := parseCommaSeparated(c.Query("q"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteSearchColdChapterHits(c.Request.Context(), terms, maxRaw)
	c.JSON(status, body)
}
