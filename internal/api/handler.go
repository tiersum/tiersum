// Package api implements HTTP handlers
// Depends only on service interfaces from ports package
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/pkg/types"
)

// Handler holds API dependencies
type Handler struct {
	docService  ports.DocumentService
	queryService ports.QueryService
	logger      *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(docService ports.DocumentService, queryService ports.QueryService, logger *zap.Logger) *Handler {
	return &Handler{
		docService:  docService,
		queryService: queryService,
		logger:      logger,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	docs := router.Group("/documents")
	{
		docs.POST("", h.CreateDocument)
		docs.GET("", h.ListDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.GET("/:id/hierarchy", h.GetHierarchy)
	}

	router.GET("/query", h.Query)
}

// CreateDocument creates a new document
func (h *Handler) CreateDocument(c *gin.Context) {
	var req types.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc, err := h.docService.Ingest(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to ingest document", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create document"})
		return
	}

	c.JSON(http.StatusCreated, types.CreateDocumentResponse{
		ID:        doc.ID,
		Title:     doc.Title,
		Format:    doc.Format,
		CreatedAt: doc.CreatedAt,
	})
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

	doc, err := h.docService.Get(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get document", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get document"})
		return
	}

	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	c.JSON(http.StatusOK, doc)
}

// GetHierarchy retrieves the hierarchical structure of a document
func (h *Handler) GetHierarchy(c *gin.Context) {
	id := c.Param("id")
	_ = c.Query("path") // TODO: Use path parameter

	h.logger.Info("Getting hierarchy", zap.String("id", id))

	// TODO: Implement hierarchy retrieval using query service
	c.JSON(http.StatusOK, types.HierarchyResponse{
		DocumentID: id,
		Hierarchy:  &types.HierarchyNode{Level: 0, Title: "Root"},
	})
}

// Query performs a hierarchical query
func (h *Handler) Query(c *gin.Context) {
	var req types.QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Depth == "" {
		req.Depth = types.TierChapter
	}

	results, err := h.queryService.Query(c.Request.Context(), req.Question, req.Depth)
	if err != nil {
		h.logger.Error("failed to query", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query"})
		return
	}

	c.JSON(http.StatusOK, types.QueryResponse{
		Question: req.Question,
		Depth:    req.Depth,
		Results:  results,
	})
}
