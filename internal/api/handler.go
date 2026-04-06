package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
)

// Handler holds API dependencies
type Handler struct {
	store  *storage.Storage
	logger *zap.Logger
}

// RegisterRoutes registers all API routes
func RegisterRoutes(router *gin.RouterGroup, store *storage.Storage, logger *zap.Logger) {
	h := &Handler{
		store:  store,
		logger: logger,
	}

	// Document routes
	docs := router.Group("/documents")
	{
		docs.POST("", h.CreateDocument)
		docs.GET("", h.ListDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.GET("/:id/hierarchy", h.GetHierarchy)
	}

	// Query routes
	router.GET("/query", h.Query)
}

// CreateDocumentRequest represents a document creation request
type CreateDocumentRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
	Format  string `json:"format" binding:"required,oneof=markdown md"`
}

// CreateDocument creates a new document
func (h *Handler) CreateDocument(c *gin.Context) {
	var req CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement document creation
	h.logger.Info("Creating document", zap.String("title", req.Title))

	c.JSON(http.StatusCreated, gin.H{
		"id":      uuid.New().String(),
		"title":   req.Title,
		"message": "Document created successfully",
	})
}

// ListDocuments lists all documents
func (h *Handler) ListDocuments(c *gin.Context) {
	// TODO: Implement document listing
	c.JSON(http.StatusOK, gin.H{
		"documents": []interface{}{},
	})
}

// GetDocument retrieves a document by ID
func (h *Handler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	// TODO: Implement document retrieval
	h.logger.Info("Getting document", zap.String("id", id))

	c.JSON(http.StatusOK, gin.H{
		"id":    id,
		"title": "Sample Document",
	})
}

// GetHierarchy retrieves the hierarchical structure of a document
func (h *Handler) GetHierarchy(c *gin.Context) {
	id := c.Param("id")
	path := c.Query("path")

	h.logger.Info("Getting hierarchy", zap.String("id", id), zap.String("path", path))

	// TODO: Implement hierarchy retrieval
	c.JSON(http.StatusOK, gin.H{
		"document_id": id,
		"path":        path,
		"hierarchy":   []interface{}{},
	})
}

// QueryRequest represents a query request
type QueryRequest struct {
	Question string `form:"question" binding:"required"`
	Depth    string `form:"depth" binding:"omitempty,oneof=document chapter paragraph source"`
}

// Query performs a hierarchical query
func (h *Handler) Query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Depth == "" {
		req.Depth = "chapter"
	}

	h.logger.Info("Querying", zap.String("question", req.Question), zap.String("depth", req.Depth))

	// TODO: Implement hierarchical query
	c.JSON(http.StatusOK, gin.H{
		"question": req.Question,
		"depth":    req.Depth,
		"results":  []interface{}{},
	})
}
