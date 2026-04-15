// Package api implements HTTP handlers
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// QuotaSnapshot exposes current hot-ingest quota for HTTP handlers.
type QuotaSnapshot interface {
	GetQuota() (used int, total int, resetAt time.Time)
}

// Handler holds API dependencies
type Handler struct {
	DocService      service.IDocumentService
	QueryService    service.IQueryService
	TopicService    service.ITopicService
	TagsService     service.ITagService
	ChaptersService service.IChapterService
	ObsService      service.IObservabilityService
	Quota           QuotaSnapshot
	OtelSpans       storage.IOtelSpanRepository
	Logger          *zap.Logger
	// ServerVersion is the release/build label (e.g. from main.Version ldflags). Empty uses moduleVersion().
	ServerVersion string
}

// NewHandler creates a new API handler
func NewHandler(
	docService service.IDocumentService,
	queryService service.IQueryService,
	topicService service.ITopicService,
	tagService service.ITagService,
	chapterService service.IChapterService,
	observabilityService service.IObservabilityService,
	quota QuotaSnapshot,
	otelSpans storage.IOtelSpanRepository,
	logger *zap.Logger,
	serverVersion string,
) *Handler {
	return &Handler{
		DocService:      docService,
		QueryService:    queryService,
		TopicService:    topicService,
		TagsService:     tagService,
		ChaptersService: chapterService,
		ObsService:      observabilityService,
		Quota:           quota,
		OtelSpans:       otelSpans,
		Logger:          logger,
		ServerVersion:   strings.TrimSpace(serverVersion),
	}
}

// RegisterRoutes registers all API routes on the given group (e.g. /api/v1 or /bff/v1 prefix from cmd).
// When traceMiddleware is non-nil, it wraps core (non-CRUD) endpoints so OpenTelemetry spans
// are recorded with configurable sampling; CRUD-style and introspection routes stay untraced.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup, traceMiddleware gin.HandlerFunc) {
	// Document CRUD
	docs := router.Group("/documents")
	{
		docs.POST("", h.CreateDocument)
		docs.GET("", h.ListDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.GET("/:id/chapters", h.GetDocumentChapters)
	}

	// Simple list reads
	router.GET("/tags", h.ListTags)
	router.GET("/topics", h.ListTopics)
	router.GET("/quota", h.GetQuota)
	router.GET("/monitoring", h.GetMonitoringSnapshot)
	router.GET("/traces", h.ListTraces)
	router.GET("/traces/:trace_id", h.GetTrace)

	core := router
	if traceMiddleware != nil {
		core = router.Group("", traceMiddleware)
	}

	core.POST("/query/progressive", h.ProgressiveQuery)
	core.POST("/topics/regroup", h.RegroupTagsIntoTopics)
	hot := core.Group("/hot")
	{
		hot.GET("/doc_summaries", h.ListHotDocumentSummariesByTags)
		hot.GET("/doc_chapters", h.ListHotDocumentChaptersByDocumentIDs)
	}
	cold := core.Group("/cold")
	{
		cold.GET("/doc_source", h.SearchColdChapterHits)
	}
}

// CreateDocument creates a new document
func (h *Handler) CreateDocument(c *gin.Context) {
	var req types.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, body := h.ExecuteCreateDocument(c.Request.Context(), req)
	c.JSON(status, body)
}

// ListDocuments lists all documents
func (h *Handler) ListDocuments(c *gin.Context) {
	status, body := h.ExecuteListDocuments(c.Request.Context())
	c.JSON(status, body)
}

// GetDocument retrieves a document by ID
func (h *Handler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	status, body := h.ExecuteGetDocument(c.Request.Context(), id)
	c.JSON(status, body)
}

// GetDocumentChapters handles GET /documents/:id/chapters (delegates to ExecuteListDocumentChaptersByDocumentID).
func (h *Handler) GetDocumentChapters(c *gin.Context) {
	docID := c.Param("id")
	status, body := h.ExecuteListDocumentChaptersByDocumentID(c.Request.Context(), docID)
	c.JSON(status, body)
}

// ProgressiveQuery handles POST /query/progressive (delegates to ExecuteProgressiveQuery: catalog tags/topics, then hot + cold paths).
func (h *Handler) ProgressiveQuery(c *gin.Context) {
	var req types.ProgressiveQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, body := h.ExecuteProgressiveQuery(c.Request.Context(), req)
	c.JSON(status, body)
}

// ListTopics lists all topics (themes).
func (h *Handler) ListTopics(c *gin.Context) {
	status, body := h.ExecuteListTopics(c.Request.Context())
	c.JSON(status, body)
}

// RegroupTagsIntoTopics runs LLM regrouping of catalog tags into topics.
func (h *Handler) RegroupTagsIntoTopics(c *gin.Context) {
	status, body := h.ExecuteRegroupTagsIntoTopics(c.Request.Context())
	c.JSON(status, body)
}

// GetQuota returns the current quota status
func (h *Handler) GetQuota(c *gin.Context) {
	status, body := h.ExecuteGetQuotaSnapshot()
	c.JSON(status, body)
}

// GetMonitoringSnapshot returns a JSON snapshot for the monitoring UI.
func (h *Handler) GetMonitoringSnapshot(c *gin.Context) {
	status, body := h.ExecuteGetMonitoringSnapshot(c.Request.Context())
	if m, ok := body.(gin.H); ok {
		// Top-level /metrics (Prometheus convention); registered in cmd without API key middleware.
		m["prometheus_metrics_path"] = "/metrics"
	}
	c.JSON(status, body)
}

// ListTraces returns recent persisted trace summaries (OpenTelemetry export).
func (h *Handler) ListTraces(c *gin.Context) {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	status, body := h.ExecuteListTraceSummaries(c.Request.Context(), limit, offset)
	c.JSON(status, body)
}

// GetTrace returns all spans for one trace id.
func (h *Handler) GetTrace(c *gin.Context) {
	tid := strings.TrimSpace(c.Param("trace_id"))
	status, body := h.ExecuteGetTraceSpans(c.Request.Context(), tid)
	c.JSON(status, body)
}
