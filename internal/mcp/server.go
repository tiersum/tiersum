package mcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
)

// Server handles MCP protocol
type Server struct {
	store  *storage.Storage
	logger *zap.Logger
	mcp    *server.MCPServer
}

// NewServer creates a new MCP server
func NewServer(store *storage.Storage, logger *zap.Logger) *Server {
	s := &Server{
		store:  store,
		logger: logger,
	}

	// Create MCP server
	s.mcp = server.NewMCPServer(
		"tiersum",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// Query tool
	queryTool := mcp.NewTool("tiersum_query",
		mcp.WithDescription("Query knowledge base with hierarchical precision"),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to query"),
		),
		mcp.WithString("depth",
			mcp.Description("Query depth: document, chapter, paragraph, or source"),
			mcp.Enum("document", "chapter", "paragraph", "source"),
		),
	)
	s.mcp.AddTool(queryTool, s.handleQuery)

	// Explore tool
	exploreTool := mcp.NewTool("tiersum_explore",
		mcp.WithDescription("Navigate document structure interactively"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID to explore"),
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform"),
			mcp.Enum("list_chapters", "get_summary", "drill_down"),
		),
	)
	s.mcp.AddTool(exploreTool, s.handleExplore)
}

// SSEHandler returns the SSE handler for MCP
func (s *Server) SSEHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.logger.Info("MCP SSE connection established")
		// TODO: Implement full SSE handling
		c.String(http.StatusOK, "MCP SSE endpoint")
	}
}

// handleQuery handles the tiersum_query tool
func (s *Server) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, ok := request.Params.Arguments["question"].(string)
	if !ok {
		return nil, fmt.Errorf("question is required")
	}

	depth, _ := request.Params.Arguments["depth"].(string)
	if depth == "" {
		depth = "chapter"
	}

	s.logger.Info("MCP query", zap.String("question", question), zap.String("depth", depth))

	// TODO: Implement query logic
	return mcp.NewToolResultText("Query results will appear here"), nil
}

// handleExplore handles the tiersum_explore tool
func (s *Server) handleExplore(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docID, ok := request.Params.Arguments["document_id"].(string)
	if !ok {
		return nil, fmt.Errorf("document_id is required")
	}

	action, ok := request.Params.Arguments["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required")
	}

	s.logger.Info("MCP explore", zap.String("document_id", docID), zap.String("action", action))

	// TODO: Implement explore logic
	return mcp.NewToolResultText("Explore results will appear here"), nil
}
