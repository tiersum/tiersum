// Package api implements HTTP handlers
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl"
	"github.com/tiersum/tiersum/pkg/types"
)

// Handler holds API dependencies
type Handler struct {
	docService   service.IDocumentService
	queryService service.IQueryService
	topicService service.ITopicService
	logger       *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(docService service.IDocumentService, queryService service.IQueryService, topicService service.ITopicService, logger *zap.Logger) *Handler {
	return &Handler{
		docService:   docService,
		queryService: queryService,
		topicService: topicService,
		logger:       logger,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	docs := router.Group("/documents")
	{
		docs.POST("", h.CreateDocument)
		docs.GET("", h.ListDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.GET("/:id/chapters", h.GetDocumentChapters)
	}

	// Topic endpoints
	topics := router.Group("/topics")
	{
		topics.POST("", h.CreateTopic)
		topics.GET("", h.ListTopics)
		topics.GET("/:id", h.GetTopic)
		topics.GET("/:id/documents", h.GetTopicDocuments)
		topics.GET("/by-tags", h.FindTopicsByTags)
	}

	router.GET("/query", h.Query)
	router.POST("/query/hierarchical", h.HierarchicalQuery)
	router.GET("/query/drill-down", h.DrillDown)
	router.GET("/query/source", h.GetSource)
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

// CreateTopicRequest represents a request to create a topic
type CreateTopicRequest struct {
	Name   string   `json:"name" binding:"required"`
	DocIDs []string `json:"document_ids" binding:"required,min=1"`
}

// CreateTopic creates a new topic from documents
func (h *Handler) CreateTopic(c *gin.Context) {
	var req CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// API created topics are marked as manual
	topic, err := h.topicService.CreateTopicFromDocuments(c.Request.Context(), req.Name, req.DocIDs, types.TopicSourceManual)
	if err != nil {
		h.logger.Error("failed to create topic", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, topic)
}

// ListTopics lists all topics
func (h *Handler) ListTopics(c *gin.Context) {
	topics, err := h.topicService.ListTopics(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list topics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list topics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// GetTopic retrieves a topic by ID
func (h *Handler) GetTopic(c *gin.Context) {
	id := c.Param("id")

	topic, err := h.topicService.GetTopic(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get topic", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get topic"})
		return
	}

	if topic == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "topic not found"})
		return
	}

	c.JSON(http.StatusOK, topic)
}

// FindTopicsByTags finds topics by tags
func (h *Handler) FindTopicsByTags(c *gin.Context) {
	tags := c.QueryArray("tags")
	if len(tags) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one tag is required"})
		return
	}

	topics, err := h.topicService.FindTopicsByTags(c.Request.Context(), tags)
	if err != nil {
		h.logger.Error("failed to find topics by tags", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find topics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// HierarchicalQuery performs progressive hierarchical query
func (h *Handler) HierarchicalQuery(c *gin.Context) {
	var req types.HierarchicalQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if req.StartTier == "" {
		req.StartTier = types.TierTopic
	}
	if req.EndTier == "" {
		req.EndTier = types.TierSource
	}
	if req.MaxResults == 0 {
		req.MaxResults = 10
	}

	// Use query service
	svc, ok := h.queryService.(*svcimpl.QuerySvc)
	if !ok {
		h.logger.Error("query service does not support hierarchical query")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not available"})
		return
	}

	response, err := svc.HierarchicalQuery(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to perform hierarchical query", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// DrillDown drills down to next level
func (h *Handler) DrillDown(c *gin.Context) {
	var req types.DrillDownRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate tier
	if !isValidTier(req.CurrentTier) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid current_tier"})
		return
	}

	// Use hierarchical query service
	svc, ok := h.queryService.(interface {
		DrillDown(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error)
	})
	if !ok {
		h.logger.Error("query service does not support drill-down")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not available"})
		return
	}

	items, err := svc.DrillDown(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to drill down", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
	})
}

// GetSource retrieves original source content
func (h *Handler) GetSource(c *gin.Context) {
	var req types.SourceQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use hierarchical query service
	svc, ok := h.queryService.(interface {
		GetSource(ctx context.Context, docID string, path string) (*types.QueryItem, error)
	})
	if !ok {
		h.logger.Error("query service does not support source retrieval")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not available"})
		return
	}

	item, err := svc.GetSource(c.Request.Context(), req.DocumentID, req.Path)
	if err != nil {
		h.logger.Error("failed to get source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

// GetTopicDocuments retrieves documents under a topic
func (h *Handler) GetTopicDocuments(c *gin.Context) {
	topicID := c.Param("id")
	if topicID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "topic id is required"})
		return
	}

	svc, ok := h.topicService.(interface {
		GetTopicDocuments(ctx context.Context, topicID string) ([]types.Document, error)
	})
	if !ok {
		h.logger.Error("topic service does not support GetTopicDocuments")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not available"})
		return
	}

	docs, err := svc.GetTopicDocuments(c.Request.Context(), topicID)
	if err != nil {
		h.logger.Error("failed to get topic documents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"documents": docs})
}

// GetDocumentChapters retrieves chapters of a document
func (h *Handler) GetDocumentChapters(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document id is required"})
		return
	}

	svc, ok := h.queryService.(interface {
		GetDocumentChapters(ctx context.Context, docID string) ([]types.Summary, error)
	})
	if !ok {
		h.logger.Error("query service does not support GetDocumentChapters")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not available"})
		return
	}

	chapters, err := svc.GetDocumentChapters(c.Request.Context(), docID)
	if err != nil {
		h.logger.Error("failed to get document chapters", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chapters": chapters})
}

// isValidTier checks if the tier is valid
func isValidTier(tier types.SummaryTier) bool {
	switch tier {
	case types.TierTopic, types.TierDocument, types.TierChapter, types.TierParagraph, types.TierSource:
		return true
	}
	return false
}
