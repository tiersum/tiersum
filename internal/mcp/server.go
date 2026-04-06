// Package mcp implements MCP protocol handlers
// Depends only on service interfaces from ports package (same as REST API)
package mcp

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/domain/service"
	"github.com/tiersum/tiersum/internal/ports"
	"github.com/tiersum/tiersum/pkg/types"
)

// Server handles MCP protocol
// Like api.Handler, it depends on service interfaces for business logic
type Server struct {
	docService   ports.DocumentService
	queryService ports.QueryService
	topicService *service.TopicSvc
	logger       *zap.Logger
	mcp          *server.MCPServer
}

// NewServer creates a new MCP server
// Accepts service interfaces (same as REST API handler)
func NewServer(docService ports.DocumentService, queryService ports.QueryService, topicService *service.TopicSvc, logger *zap.Logger) *Server {
	s := &Server{
		docService:   docService,
		queryService: queryService,
		topicService: topicService,
		logger:       logger,
	}

	// Create MCP server
	s.mcp = server.NewMCPServer(
		"tiersum",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
	)

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// Query tool - delegates to queryService (same as REST API)
	queryTool := mcp.NewTool("tiersum_query",
		mcp.WithDescription("Query knowledge base with hierarchical precision"),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to query"),
		),
		mcp.WithString("depth",
			mcp.Description("Query depth: topic, document, chapter, paragraph, or source"),
			mcp.Enum("topic", "document", "chapter", "paragraph", "source"),
		),
	)
	s.mcp.AddTool(queryTool, s.handleQuery)

	// GetDocument tool - delegates to docService (same as REST API)
	getDocTool := mcp.NewTool("tiersum_get_document",
		mcp.WithDescription("Retrieve a document by ID"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID to retrieve"),
		),
	)
	s.mcp.AddTool(getDocTool, s.handleGetDocument)

	// ListTopics tool - lists all topics
	listTopicsTool := mcp.NewTool("tiersum_list_topics",
		mcp.WithDescription("List all topic summaries"),
	)
	s.mcp.AddTool(listTopicsTool, s.handleListTopics)

	// GetTopic tool - retrieves a topic by ID
	getTopicTool := mcp.NewTool("tiersum_get_topic",
		mcp.WithDescription("Retrieve a topic summary by ID"),
		mcp.WithString("topic_id",
			mcp.Required(),
			mcp.Description("The topic ID to retrieve"),
		),
	)
	s.mcp.AddTool(getTopicTool, s.handleGetTopic)
}

// GetMCPServer returns the underlying MCP server for SSE handling
func (s *Server) GetMCPServer() *server.MCPServer {
	return s.mcp
}

// SSEHandler returns the SSE handler for MCP
// Uses server.SSEServer from mcp-go for proper SSE handling
func (s *Server) SSEHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.logger.Info("MCP SSE connection established")
		// TODO: Use server.NewSSEServer(s.mcp, baseURL) for full implementation
		c.String(200, "MCP SSE endpoint - not fully implemented")
	}
}

// handleQuery handles the tiersum_query tool
// Delegates to queryService.Query (same business logic as REST API)
func (s *Server) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, ok := request.Params.Arguments["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	depthStr, _ := request.Params.Arguments["depth"].(string)
	if depthStr == "" {
		depthStr = "chapter"
	}

	// Parse depth string to SummaryTier
	depth := types.SummaryTier(depthStr)
	if !isValidDepth(depth) {
		return nil, fmt.Errorf("invalid depth: %s", depthStr)
	}

	s.logger.Info("MCP query", zap.String("question", question), zap.String("depth", string(depth)))

	// Call query service (shared business logic with REST API)
	results, err := s.queryService.Query(ctx, question, depth)
	if err != nil {
		s.logger.Error("query failed", zap.Error(err))
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Format results
	resultText := formatQueryResults(question, depth, results)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetDocument handles the tiersum_get_document tool
// Delegates to docService.Get (same business logic as REST API)
func (s *Server) handleGetDocument(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docID, ok := request.Params.Arguments["document_id"].(string)
	if !ok || docID == "" {
		return nil, fmt.Errorf("document_id is required")
	}

	s.logger.Info("MCP get document", zap.String("document_id", docID))

	// Call document service (shared business logic with REST API)
	doc, err := s.docService.Get(ctx, docID)
	if err != nil {
		s.logger.Error("failed to get document", zap.String("id", docID), zap.Error(err))
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if doc == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Document not found: %s", docID)), nil
	}

	// Format document info
	resultText := formatDocument(doc)
	return mcp.NewToolResultText(resultText), nil
}

// isValidDepth checks if the depth is valid
func isValidDepth(depth types.SummaryTier) bool {
	switch depth {
	case types.TierTopic, types.TierDocument, types.TierChapter, types.TierParagraph, types.TierSource:
		return true
	}
	return false
}

// handleListTopics handles the tiersum_list_topics tool
func (s *Server) handleListTopics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.Info("MCP list topics")

	topics, err := s.topicService.ListTopics(ctx)
	if err != nil {
		s.logger.Error("failed to list topics", zap.Error(err))
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	resultText := formatTopicList(topics)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetTopic handles the tiersum_get_topic tool
func (s *Server) handleGetTopic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topicID, ok := request.Params.Arguments["topic_id"].(string)
	if !ok || topicID == "" {
		return nil, fmt.Errorf("topic_id is required")
	}

	s.logger.Info("MCP get topic", zap.String("topic_id", topicID))

	topic, err := s.topicService.GetTopic(ctx, topicID)
	if err != nil {
		s.logger.Error("failed to get topic", zap.String("id", topicID), zap.Error(err))
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	if topic == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Topic not found: %s", topicID)), nil
	}

	resultText := formatTopic(topic)
	return mcp.NewToolResultText(resultText), nil
}

// formatTopicList formats a list of topics for display
func formatTopicList(topics []types.TopicSummary) string {
	if len(topics) == 0 {
		return "No topics found."
	}

	text := "Available Topics:\n\n"
	for i, t := range topics {
		text += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, t.Name, t.ID)
		if t.Description != "" {
			text += fmt.Sprintf("   %s\n", t.Description)
		}
		if len(t.Tags) > 0 {
			text += fmt.Sprintf("   Tags: %v\n", t.Tags)
		}
		text += "\n"
	}
	return text
}

// formatTopic formats a single topic for display
func formatTopic(topic *types.TopicSummary) string {
	text := fmt.Sprintf(
		"Topic: %s\nID: %s\n",
		topic.Name,
		topic.ID,
	)

	if topic.Description != "" {
		text += fmt.Sprintf("Description: %s\n", topic.Description)
	}

	if len(topic.Tags) > 0 {
		text += fmt.Sprintf("Tags: %v\n", topic.Tags)
	}

	text += fmt.Sprintf("\nSummary:\n%s\n", topic.Summary)

	if len(topic.DocumentIDs) > 0 {
		text += fmt.Sprintf("\nAssociated Documents (%d):\n", len(topic.DocumentIDs))
		for i, docID := range topic.DocumentIDs {
			text += fmt.Sprintf("  %d. %s\n", i+1, docID)
		}
	}

	return text
}

// formatQueryResults formats query results for display
func formatQueryResults(question string, depth types.SummaryTier, results []types.QueryResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("Query: %s\nDepth: %s\n\nNo results found.", question, depth)
	}

	text := fmt.Sprintf("Query: %s\nDepth: %s\n\nResults:\n", question, depth)
	for i, r := range results {
		text += fmt.Sprintf("\n%d. %s\n   %s\n", i+1, r.DocumentTitle, r.Content)
	}
	return text
}

// formatDocument formats document info for display
func formatDocument(doc *types.Document) string {
	return fmt.Sprintf(
		"Document: %s\nID: %s\nFormat: %s\nCreated: %s\n\nContent Preview:\n%s",
		doc.Title,
		doc.ID,
		doc.Format,
		doc.CreatedAt.Format("2006-01-02 15:04:05"),
		truncateString(doc.Content, 500),
	)
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
