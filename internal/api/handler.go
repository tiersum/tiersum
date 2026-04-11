// Package api implements HTTP handlers
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
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
	TagGroupService service.ITagGroupService
	Retrieval       service.IRetrievalService
	Quota           QuotaSnapshot
	Logger          *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(
	docService service.IDocumentService,
	queryService service.IQueryService,
	tagGroupService service.ITagGroupService,
	retrieval service.IRetrievalService,
	quota QuotaSnapshot,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		DocService:      docService,
		QueryService:    queryService,
		TagGroupService: tagGroupService,
		Retrieval:       retrieval,
		Quota:           quota,
		Logger:          logger,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// Document endpoints
	docs := router.Group("/documents")
	{
		docs.POST("", h.CreateDocument)
		docs.GET("", h.ListDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.GET("/:id/chapters", h.GetDocumentChapters)
	}

	// Query endpoints
	router.POST("/query/progressive", h.ProgressiveQuery)

	// Tag endpoints
	router.GET("/tags", h.ListTags)
	router.GET("/tags/groups", h.ListTagGroups)
	router.POST("/tags/group", h.TriggerTagGroup)

	// Hot / cold retrieval (summaries and sources; cold uses cold index, not tags)
	hot := router.Group("/hot")
	{
		hot.GET("/doc_summaries", h.HotDocSummaries)
		hot.GET("/doc_chapters", h.HotDocChapters)
		hot.GET("/doc_source", h.HotDocSource)
	}
	cold := router.Group("/cold")
	{
		cold.GET("/doc_source", h.ColdDocSource)
	}

	// Document summaries endpoint
	router.GET("/documents/:id/summaries", h.GetDocumentSummaries)

	// Quota endpoint
	router.GET("/quota", h.GetQuota)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// CreateDocument creates a new document
func (h *Handler) CreateDocument(c *gin.Context) {
	var req types.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, body := h.ExecuteIngestDocument(c.Request.Context(), req)
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

// GetDocumentChapters retrieves chapters of a document
func (h *Handler) GetDocumentChapters(c *gin.Context) {
	docID := c.Param("id")
	status, body := h.ExecuteGetDocumentChapters(c.Request.Context(), docID)
	c.JSON(status, body)
}

// ProgressiveQuery performs the new two-level tag-based progressive query
func (h *Handler) ProgressiveQuery(c *gin.Context) {
	var req types.ProgressiveQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, body := h.ExecuteProgressiveQuery(c.Request.Context(), req)
	c.JSON(status, body)
}

// ListTagGroups lists all tag groups (Level 1 categories)
func (h *Handler) ListTagGroups(c *gin.Context) {
	status, body := h.ExecuteListTagGroups(c.Request.Context())
	c.JSON(status, body)
}

// TriggerTagGroup manually triggers tag grouping
func (h *Handler) TriggerTagGroup(c *gin.Context) {
	status, body := h.ExecuteTriggerTagGroup(c.Request.Context())
	c.JSON(status, body)
}

// GetDocumentSummaries retrieves all summaries for a document
func (h *Handler) GetDocumentSummaries(c *gin.Context) {
	id := c.Param("id")
	status, body := h.ExecuteGetDocumentSummaries(c.Request.Context(), id)
	c.JSON(status, body)
}

// GetQuota returns the current quota status
func (h *Handler) GetQuota(c *gin.Context) {
	status, body := h.ExecuteGetQuota()
	c.JSON(status, body)
}
