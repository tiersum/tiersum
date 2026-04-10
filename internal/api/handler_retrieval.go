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

// ListTags handles GET /tags with optional group_ids and max_results.
func (h *Handler) ListTags(c *gin.Context) {
	ctx := c.Request.Context()
	groupIDs := parseCommaSeparated(c.Query("group_ids"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteListTags(ctx, groupIDs, maxRaw)
	c.JSON(status, body)
}

// HotDocSummaries handles GET /hot/doc_summaries — hot/warming docs matching tags; document-level summary only (no body).
func (h *Handler) HotDocSummaries(c *gin.Context) {
	tags := parseCommaSeparated(c.Query("tags"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteHotDocSummaries(c.Request.Context(), tags, maxRaw)
	c.JSON(status, body)
}

// HotDocChapters handles GET /hot/doc_chapters — chapter-level summaries (not source) per document.
func (h *Handler) HotDocChapters(c *gin.Context) {
	docIDs := parseCommaSeparated(c.Query("doc_ids"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteHotDocChapters(c.Request.Context(), docIDs, maxRaw)
	c.JSON(status, body)
}

// HotDocSource handles GET /hot/doc_source — original chapter text for given chapter paths.
func (h *Handler) HotDocSource(c *gin.Context) {
	paths := parseCommaSeparated(c.Query("chapter_paths"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteHotDocSource(c.Request.Context(), paths, maxRaw)
	c.JSON(status, body)
}

// ColdDocSource handles GET /cold/doc_source — locate query terms in cold docs via memory index; returns context snippets (no tag filter).
func (h *Handler) ColdDocSource(c *gin.Context) {
	terms := parseCommaSeparated(c.Query("q"))
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteColdDocSource(c.Request.Context(), terms, maxRaw)
	c.JSON(status, body)
}
