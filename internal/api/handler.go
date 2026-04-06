// Package api implements HTTP handlers
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// Handler holds API dependencies
type Handler struct {
	DocService           service.IDocumentService
	QueryService         service.IQueryService
	TagGroupingService service.ITagGroupingService
	Logger               *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(
	docService service.IDocumentService,
	queryService service.IQueryService,
	tagGroupingService service.ITagGroupingService,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		DocService:           docService,
		QueryService:         queryService,
		TagGroupingService: tagGroupingService,
		Logger:               logger,
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
	router.GET("/query", h.Query)
	router.POST("/query/progressive", h.ProgressiveQuery)

	// Tag cluster endpoints
	router.GET("/tags/clusters", h.ListTagGroups)
	router.POST("/tags/cluster", h.TriggerTagGrouping)
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
	// For now, return empty list
	// TODO: Implement pagination
	c.JSON(http.StatusOK, gin.H{
		"documents": []types.Document{},
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
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// Query performs a query (legacy interface)
func (h *Handler) Query(c *gin.Context) {
	var req types.QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Depth == "" {
		req.Depth = types.TierChapter
	}

	results, err := h.QueryService.Query(c.Request.Context(), req.Question, req.Depth)
	if err != nil {
		h.Logger.Error("failed to query", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query"})
		return
	}

	c.JSON(http.StatusOK, types.QueryResponse{
		Question: req.Question,
		Depth:    req.Depth,
		Results:  results,
	})
}

// ProgressiveQuery performs the new two-level tag-based progressive query
func (h *Handler) ProgressiveQuery(c *gin.Context) {
	var req types.ProgressiveQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
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

// ListTagGroups lists all tag clusters (Level 1 categories)
func (h *Handler) ListTagGroups(c *gin.Context) {
	if h.TagGroupingService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tag clustering service not available"})
		return
	}

	clusters, err := h.TagGroupingService.GetL1Clusters(c.Request.Context())
	if err != nil {
		h.Logger.Error("failed to list tag clusters", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"clusters": clusters})
}

// TriggerTagGrouping manually triggers tag clustering
func (h *Handler) TriggerTagGrouping(c *gin.Context) {
	if h.TagGroupingService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tag clustering service not available"})
		return
	}

	if err := h.TagGroupingService.ClusterTags(c.Request.Context()); err != nil {
		h.Logger.Error("failed to cluster tags", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag clustering triggered"})
}
