package api

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/tiersum/tiersum/internal/telemetry"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
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
		if errors.Is(err, service.ErrIngestValidation) {
			h.Logger.Info("ingest validation rejected", zap.Error(err))
			return http.StatusBadRequest, gin.H{"error": err.Error()}
		}
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

	// Cold documents: DB chapter-tier rows (if any) are not the source of truth for the detail UI.
	// Use the same markdown chapter split as the cold index so merged sections (e.g. ##2 + ###2.1 + ###2.2)
	// return full text including all subsections; stale or partial summary rows would omit e.g. 2.2 tables.
	if doc.Status == types.DocStatusCold {
		out := make([]gin.H, 0)
		mdChapters, ferr := h.Retrieval.MarkdownChaptersForDocument(ctx, doc)
		if ferr != nil {
			h.Logger.Warn("markdown chapters for cold document", zap.String("id", docID), zap.Error(ferr))
		}
		for _, c := range mdChapters {
			out = append(out, gin.H{
				"path":    c.Path,
				"title":   c.Title,
				"summary": c.Content,
			})
		}
		return http.StatusOK, gin.H{"document_id": docID, "chapters": out}
	}

	chapters, err := h.Retrieval.ListChapterSummariesForDocument(ctx, docID)
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
	if len(out) == 0 {
		fallback, ferr := h.Retrieval.MarkdownChaptersForDocument(ctx, doc)
		if ferr != nil {
			h.Logger.Warn("markdown chapters fallback failed", zap.String("id", docID), zap.Error(ferr))
		}
		for _, c := range fallback {
			out = append(out, gin.H{
				"path":    c.Path,
				"title":   c.Title,
				"summary": c.Content,
			})
		}
	}
	return http.StatusOK, gin.H{"document_id": docID, "chapters": out}
}

// ExecuteGetDocumentSummaries matches GET /api/v1/documents/:id/summaries.
func (h *Handler) ExecuteGetDocumentSummaries(ctx context.Context, id string) (int, any) {
	summaries, err := h.Retrieval.ListSummariesForDocument(ctx, id)
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
	if telemetry.GlobalTracerActive() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if flushErr := telemetry.FlushSpans(flushCtx); flushErr != nil {
			h.Logger.Warn("telemetry flush after progressive query", zap.Error(flushErr))
		}
	}
	if err != nil {
		h.Logger.Error("failed to perform progressive query", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	return http.StatusOK, response
}

// ExecuteListTopics matches GET /api/v1/topics.
func (h *Handler) ExecuteListTopics(ctx context.Context) (int, any) {
	if h.TopicService == nil {
		return http.StatusServiceUnavailable, gin.H{"error": "topic service not available"}
	}
	topics, err := h.TopicService.ListTopics(ctx)
	if err != nil {
		h.Logger.Error("failed to list topics", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	if topics == nil {
		topics = []types.Topic{}
	}
	return http.StatusOK, gin.H{"topics": topics}
}

// ExecuteTriggerTopicRegroup matches POST /api/v1/topics/regroup.
func (h *Handler) ExecuteTriggerTopicRegroup(ctx context.Context) (int, any) {
	if h.TopicService == nil {
		return http.StatusServiceUnavailable, gin.H{"error": "topic service not available"}
	}
	if err := h.TopicService.RegroupTags(ctx); err != nil {
		h.Logger.Error("failed to regroup tags into topics", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	return http.StatusOK, gin.H{"message": "topic regrouping completed"}
}

// ExecuteListTags matches GET /api/v1/tags (topic_ids optional; max_results optional string like the query param).
func (h *Handler) ExecuteListTags(ctx context.Context, topicIDs []string, maxResultsQuery string) (int, any) {
	byTopicLimit := 0
	listAllCap := 0
	if len(topicIDs) > 0 {
		byTopicLimit = parseMaxResultsFromString(maxResultsQuery, 100, 10000)
	} else {
		if raw := strings.TrimSpace(maxResultsQuery); raw != "" {
			if capN, e := strconv.Atoi(raw); e == nil && capN > 0 {
				listAllCap = capN
				if listAllCap > 10000 {
					listAllCap = 10000
				}
			}
		}
	}
	tags, err := h.Retrieval.ListTags(ctx, topicIDs, byTopicLimit, listAllCap)
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
	if len(tags) == 0 {
		return http.StatusBadRequest, gin.H{"error": "tags query parameter is required (comma-separated)"}
	}
	n := parseMaxResultsFromString(maxResultsQuery, 1000, 10000)
	docs, sumRows, err := h.Retrieval.HotDocumentsWithDocSummaries(ctx, tags, n)
	if err != nil {
		h.Logger.Error("hot doc_summaries", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	if len(docs) == 0 {
		return http.StatusOK, gin.H{"items": []any{}}
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
	byDoc, err := h.Retrieval.ChapterSummariesByDocumentIDs(ctx, docIDs)
	if err != nil {
		h.Logger.Error("hot doc_chapters", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}
	docsOut := make([]gin.H, 0, len(docIDs))
	for _, docID := range docIDs {
		chapters := byDoc[docID]
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
	summaries, err := h.Retrieval.ListSourcesByChapterPaths(ctx, paths)
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
	if len(terms) == 0 {
		return http.StatusBadRequest, gin.H{"error": "q query parameter is required (comma-separated keywords)"}
	}
	n := parseMaxResultsFromString(maxResultsQuery, 100, 500)
	queryText := strings.Join(terms, " ")
	results, err := h.Retrieval.SearchColdByQuery(ctx, queryText, n)
	if errors.Is(err, service.ErrColdIndexUnavailable) {
		return http.StatusServiceUnavailable, gin.H{"error": "cold document index not available"}
	}
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
			"context":     r.Content,
		}
		if r.Path != "" {
			entry["path"] = r.Path
		}
		if r.Source != "" {
			entry["source"] = r.Source
		}
		items = append(items, entry)
	}
	return http.StatusOK, gin.H{"items": items}
}

// ExecuteMonitoring matches GET …/monitoring (e.g. /api/v1 or /bff/v1) — JSON snapshot for dashboards (not Prometheus format).
func (h *Handler) ExecuteMonitoring(ctx context.Context) (int, any) {
	docCounts := map[string]int{
		"total":   0,
		"hot":     0,
		"cold":    0,
		"warming": 0,
	}
	docs, err := h.DocService.List(ctx)
	if err != nil {
		h.Logger.Warn("monitoring: list documents", zap.Error(err))
	} else {
		for _, d := range docs {
			docCounts["total"]++
			switch d.Status {
			case types.DocStatusHot:
				docCounts["hot"]++
			case types.DocStatusCold:
				docCounts["cold"]++
			case types.DocStatusWarming:
				docCounts["warming"]++
			}
		}
	}

	var quota any
	if h.Quota == nil {
		quota = gin.H{"used": 0, "total": 100, "reset_at": nil}
	} else {
		used, total, resetAt := h.Quota.GetQuota()
		var resetStr any
		if !resetAt.IsZero() {
			resetStr = resetAt.UTC().Format(time.RFC3339)
		} else {
			resetStr = nil
		}
		quota = gin.H{"used": used, "total": total, "reset_at": resetStr}
	}

	coldApprox := 0
	vector := gin.H{
		"hnsw_nodes":               0,
		"vector_dim":               0,
		"hnsw_m":                   0,
		"hnsw_ef_search":           0,
		"text_embedder_configured": false,
	}
	inverted := gin.H{
		"bleve_doc_count": 0,
		"storage_backend": "",
		"text_analyzer":   "",
	}
	if h.Retrieval != nil {
		coldApprox = h.Retrieval.ApproxColdIndexEntries()
		vs := h.Retrieval.ColdIndexVectorStats()
		vector = gin.H{
			"hnsw_nodes":               vs.HNSWNodes,
			"vector_dim":               vs.VectorDim,
			"hnsw_m":                   vs.HNSWM,
			"hnsw_ef_search":           vs.HNSWEfSearch,
			"text_embedder_configured": vs.TextEmbedderConfigured,
		}
		is := h.Retrieval.ColdIndexInvertedStats()
		inverted = gin.H{
			"bleve_doc_count": is.BleveDocCount,
			"storage_backend": is.StorageBackend,
			"text_analyzer":   is.TextAnalyzer,
		}
	}

	ver := strings.TrimSpace(h.ServerVersion)
	if ver == "" {
		ver = moduleVersion()
	}

	return http.StatusOK, gin.H{
		"server": gin.H{
			"version": ver,
		},
		"go": gin.H{
			"version":    runtime.Version(),
			"goos":       runtime.GOOS,
			"goarch":     runtime.GOARCH,
			"compiler":   runtime.Compiler,
			"num_cpu":    runtime.NumCPU(),
			"gomaxprocs": runtime.GOMAXPROCS(0),
		},
		"documents": docCounts,
		"cold_index": gin.H{
			"approx_chapters": coldApprox,
			"inverted":        inverted,
			"vector":          vector,
		},
		"telemetry": gin.H{
			"http_tracing_active":       telemetry.GlobalTracerActive(),
			"progressive_debug_allowed": viper.GetBool("query.allow_progressive_debug"),
		},
		"quota": quota,
	}
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

// ExecuteListTraces matches GET /api/v1/traces.
func (h *Handler) ExecuteListTraces(ctx context.Context, limit, offset int) (int, any) {
	if h.OtelSpans == nil {
		return http.StatusOK, gin.H{"traces": []types.OtelTraceSummary{}}
	}
	tr, err := h.OtelSpans.ListTraceSummaries(ctx, limit, offset)
	if err != nil {
		h.Logger.Error("list traces failed", zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to list traces"}
	}
	if tr == nil {
		tr = []types.OtelTraceSummary{}
	}
	return http.StatusOK, gin.H{"traces": tr}
}

// ExecuteGetTrace matches GET /api/v1/traces/:trace_id.
func (h *Handler) ExecuteGetTrace(ctx context.Context, traceID string) (int, any) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return http.StatusBadRequest, gin.H{"error": "trace_id is required"}
	}
	if h.OtelSpans == nil {
		return http.StatusNotFound, gin.H{"error": "trace store not configured"}
	}
	spans, err := h.OtelSpans.ListSpansByTraceID(ctx, traceID)
	if err != nil {
		h.Logger.Error("get trace failed", zap.String("trace_id", traceID), zap.Error(err))
		return http.StatusInternalServerError, gin.H{"error": "failed to load trace"}
	}
	if len(spans) == 0 {
		return http.StatusNotFound, gin.H{"error": "trace not found"}
	}
	return http.StatusOK, gin.H{"trace_id": traceID, "spans": spans}
}
