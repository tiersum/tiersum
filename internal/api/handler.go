// Package api implements HTTP handlers
package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	TagGroupService service.ITagGroupService
	TagRepo         storage.ITagRepository
	SummaryRepo     storage.ISummaryRepository
	DocRepo         storage.IDocumentRepository
	MemIndex        storage.IInMemoryIndex
	Quota           QuotaSnapshot
	Logger          *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(
	docService service.IDocumentService,
	queryService service.IQueryService,
	tagGroupService service.ITagGroupService,
	tagRepo storage.ITagRepository,
	summaryRepo storage.ISummaryRepository,
	docRepo storage.IDocumentRepository,
	memIndex storage.IInMemoryIndex,
	quota QuotaSnapshot,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		DocService:      docService,
		QueryService:    queryService,
		TagGroupService: tagGroupService,
		TagRepo:         tagRepo,
		SummaryRepo:     summaryRepo,
		DocRepo:         docRepo,
		MemIndex:        memIndex,
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

	// Hot / cold retrieval (summaries and sources; cold uses memory index, not tags)
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

	doc, err := h.DocService.Ingest(c.Request.Context(), req)
	if err != nil {
		h.Logger.Error("failed to ingest document", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create document"})
		return
	}

	c.JSON(http.StatusCreated, doc)
}

// ListDocuments lists all documents
func (h *Handler) ListDocuments(c *gin.Context) {
	docs, err := h.DocService.List(c.Request.Context())
	if err != nil {
		h.Logger.Error("failed to list documents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list documents"})
		return
	}
	// Ensure we return an empty array instead of null
	if docs == nil {
		docs = []types.Document{}
	}
	c.JSON(http.StatusOK, gin.H{
		"documents": docs,
	})
}

// GetDocument retrieves a document by ID
func (h *Handler) GetDocument(c *gin.Context) {
	id := c.Param("id")

	doc, err := h.DocService.Get(c.Request.Context(), id)
	if err != nil {
		h.Logger.Error("failed to get document", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get document"})
		return
	}

	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	c.JSON(http.StatusOK, doc)
}

// GetDocumentChapters retrieves chapters of a document
func (h *Handler) GetDocumentChapters(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document id is required"})
		return
	}

	// This would need to be implemented in the query service
	doc, err := h.DocService.Get(c.Request.Context(), docID)
	if err != nil {
		h.Logger.Error("failed to get document", zap.String("id", docID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get document"})
		return
	}
	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	chapters, err := h.SummaryRepo.QueryByTierAndPrefix(c.Request.Context(), types.TierChapter, docID+"/")
	if err != nil {
		h.Logger.Error("failed to list chapters", zap.String("id", docID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list chapters"})
		return
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

	c.JSON(http.StatusOK, gin.H{"document_id": docID, "chapters": out})
}

// ProgressiveQuery performs the new two-level tag-based progressive query
func (h *Handler) ProgressiveQuery(c *gin.Context) {
	var req types.ProgressiveQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	response, err := h.QueryService.ProgressiveQuery(c.Request.Context(), req)
	if err != nil {
		h.Logger.Error("failed to perform progressive query", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListTagGroups lists all tag groups (Level 1 categories)
func (h *Handler) ListTagGroups(c *gin.Context) {
	if h.TagGroupService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tag grouping service not available"})
		return
	}

	groups, err := h.TagGroupService.GetL1Groups(c.Request.Context())
	if err != nil {
		h.Logger.Error("failed to list tag groups", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Ensure we return an empty array instead of null
	if groups == nil {
		groups = []types.TagGroup{}
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// TriggerTagGroup manually triggers tag grouping
func (h *Handler) TriggerTagGroup(c *gin.Context) {
	if h.TagGroupService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tag grouping service not available"})
		return
	}

	if err := h.TagGroupService.GroupTags(c.Request.Context()); err != nil {
		h.Logger.Error("failed to group tags", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag grouping triggered"})
}

// GetDocumentSummaries retrieves all summaries for a document
func (h *Handler) GetDocumentSummaries(c *gin.Context) {
	id := c.Param("id")

	summaries, err := h.SummaryRepo.GetByDocument(c.Request.Context(), id)
	if err != nil {
		h.Logger.Error("failed to get document summaries", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"summaries": summaries})
}

// GetQuota returns the current quota status
func (h *Handler) GetQuota(c *gin.Context) {
	if h.Quota == nil {
		c.JSON(http.StatusOK, gin.H{
			"used":     0,
			"total":    100,
			"reset_at": nil,
		})
		return
	}
	used, total, resetAt := h.Quota.GetQuota()
	c.JSON(http.StatusOK, gin.H{
		"used":     used,
		"total":    total,
		"reset_at": resetAt,
	})
}
