package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

type Handler struct {
	DocService         service.IDocumentService
	QueryService       service.IQueryService
	TopicService       service.ITopicService
	TagsService        service.ITagService
	ChaptersService    service.IChapterService
	ObsService         service.IObservabilityService
	TraceService       service.ITraceService
	MaintenanceService service.IDocumentMaintenanceService
	Logger             *zap.Logger
	ServerVersion      string
}

func NewHandler(
	docService service.IDocumentService,
	queryService service.IQueryService,
	topicService service.ITopicService,
	tagService service.ITagService,
	chapterService service.IChapterService,
	observabilityService service.IObservabilityService,
	traceService service.ITraceService,
	maintenanceService service.IDocumentMaintenanceService,
	logger *zap.Logger,
	serverVersion string,
) *Handler {
	return &Handler{
		DocService:         docService,
		QueryService:       queryService,
		TopicService:       topicService,
		TagsService:        tagService,
		ChaptersService:    chapterService,
		ObsService:         observabilityService,
		TraceService:       traceService,
		MaintenanceService: maintenanceService,
		Logger:             logger,
		ServerVersion:      strings.TrimSpace(serverVersion),
	}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup, traceMiddleware gin.HandlerFunc) {
	docs := router.Group("/documents")
	{
		docs.POST("", h.CreateDocument)
		docs.GET("", h.ListDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.GET("/:id/chapters", h.GetDocumentChapters)
		docs.POST("/:id/promote", h.PromoteDocument)
	}

	router.GET("/tags", h.ListTags)
	router.GET("/topics", h.ListTopics)
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
		cold.GET("/chapter_hits", h.SearchColdChapterHits)
		cold.GET("/doc_source", h.SearchColdDocSource)
	}
}

func (h *Handler) CreateDocument(c *gin.Context) {
	var req types.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, body := h.ExecuteCreateDocument(c.Request.Context(), req)
	c.JSON(status, body)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	maxRaw := strings.TrimSpace(c.Query(maxResultsQueryParam))
	status, body := h.ExecuteListDocuments(c.Request.Context(), maxRaw)
	c.JSON(status, body)
}

func (h *Handler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	status, body := h.ExecuteGetDocument(c.Request.Context(), id)
	c.JSON(status, body)
}

func (h *Handler) GetDocumentChapters(c *gin.Context) {
	docID := c.Param("id")
	status, body := h.ExecuteListDocumentChaptersByDocumentID(c.Request.Context(), docID)
	c.JSON(status, body)
}

func (h *Handler) PromoteDocument(c *gin.Context) {
	docID := c.Param("id")
	status, body := h.ExecutePromoteDocument(c.Request.Context(), docID)
	c.JSON(status, body)
}

func (h *Handler) ProgressiveQuery(c *gin.Context) {
	var req types.ProgressiveQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, body := h.ExecuteProgressiveQuery(c.Request.Context(), req)
	c.JSON(status, body)
}

func (h *Handler) ListTopics(c *gin.Context) {
	status, body := h.ExecuteListTopics(c.Request.Context())
	c.JSON(status, body)
}

func (h *Handler) RegroupTagsIntoTopics(c *gin.Context) {
	status, body := h.ExecuteRegroupTagsIntoTopics(c.Request.Context())
	c.JSON(status, body)
}

func (h *Handler) GetMonitoringSnapshot(c *gin.Context) {
	status, body := h.ExecuteGetMonitoringSnapshot(c.Request.Context())
	if m, ok := body.(gin.H); ok {
		m["prometheus_metrics_path"] = "/metrics"
	}
	c.JSON(status, body)
}

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

func (h *Handler) GetTrace(c *gin.Context) {
	tid := strings.TrimSpace(c.Param("trace_id"))
	status, body := h.ExecuteGetTraceSpans(c.Request.Context(), tid)
	c.JSON(status, body)
}
