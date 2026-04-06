// Package api implements API layer
// Includes REST API and MCP protocol handlers
package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// MCPServer handles MCP protocol
type MCPServer struct {
	docService           service.IDocumentService
	queryService         service.IQueryService
	tagGroupService    service.ITagGroupService
	logger               *zap.Logger
	mcp                  *mcpserver.MCPServer
}

// NewMCPServer creates a new MCP server
func NewMCPServer(
	docService service.IDocumentService,
	queryService service.IQueryService,
	tagGroupService service.ITagGroupService,
	logger *zap.Logger,
) *MCPServer {
	s := &MCPServer{
		docService:      docService,
		queryService:    queryService,
		tagGroupService: tagGroupService,
		logger:          logger,
	}

	// Create MCP server
	s.mcp = mcpserver.NewMCPServer(
		"tiersum",
		"2.0.0",
		mcpserver.WithResourceCapabilities(true, true),
	)

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() {
	// Query tool - simple query
	queryTool := mcp.NewTool("tiersum_query",
		mcp.WithDescription("Query knowledge base for relevant content"),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to query"),
		),
		mcp.WithString("depth",
			mcp.Description("Query depth: document, chapter, or source"),
			mcp.Enum("document", "chapter", "source"),
		),
	)
	s.mcp.AddTool(queryTool, s.handleQuery)

	// Progressive Query tool - new two-level tag-based query
	progressiveQueryTool := mcp.NewTool("tiersum_progressive_query",
		mcp.WithDescription("Perform progressive query using two-level tag hierarchy (recommended)"),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to query"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum documents to query (default: 100)"),
		),
	)
	s.mcp.AddTool(progressiveQueryTool, s.handleProgressiveQuery)

	// GetDocument tool
	getDocTool := mcp.NewTool("tiersum_get_document",
		mcp.WithDescription("Retrieve a document by ID"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID to retrieve"),
		),
	)
	s.mcp.AddTool(getDocTool, s.handleGetDocument)

	// ListTagGroups tool
	listGroupsTool := mcp.NewTool("tiersum_list_tag_groups",
		mcp.WithDescription("List all tag groups (Level 1 categories)"),
	)
	s.mcp.AddTool(listGroupsTool, s.handleListTagGroups)

	// GetTagsByGroup tool
	getTagsByGroupTool := mcp.NewTool("tiersum_get_tags_by_group",
		mcp.WithDescription("Get all tags (Level 2) belonging to a specific group"),
		mcp.WithString("group_id",
			mcp.Required(),
			mcp.Description("The group ID"),
		),
	)
	s.mcp.AddTool(getTagsByGroupTool, s.handleGetTagsByGroup)

	// TriggerTagGroup tool
	triggerGroupingTool := mcp.NewTool("tiersum_trigger_tag_grouping",
		mcp.WithDescription("Manually trigger tag grouping (normally runs automatically every 30 minutes)"),
	)
	s.mcp.AddTool(triggerGroupingTool, s.handleTriggerTagGroup)
}

// GetMCPServer returns the underlying MCP server for SSE handling
func (s *MCPServer) GetMCPServer() *mcpserver.MCPServer {
	return s.mcp
}

// SSEHandler returns the SSE handler for MCP
func (s *MCPServer) SSEHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.logger.Info("MCP SSE connection established")
		// TODO: Use mcpserver.NewSSEServer(s.mcp, baseURL) for full implementation
		c.String(200, "MCP SSE endpoint - not fully implemented")
	}
}

// handleQuery handles the tiersum_query tool
func (s *MCPServer) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, ok := request.Params.Arguments["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	depthStr, _ := request.Params.Arguments["depth"].(string)
	if depthStr == "" {
		depthStr = "chapter"
	}

	depth := types.SummaryTier(depthStr)

	s.logger.Info("MCP query", zap.String("question", question), zap.String("depth", string(depth)))

	results, err := s.queryService.Query(ctx, question, depth)
	if err != nil {
		s.logger.Error("query failed", zap.Error(err))
		return nil, fmt.Errorf("query failed: %w", err)
	}

	resultText := formatQueryResults(question, depth, results)
	return mcp.NewToolResultText(resultText), nil
}

// handleProgressiveQuery handles the tiersum_progressive_query tool
func (s *MCPServer) handleProgressiveQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, ok := request.Params.Arguments["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	maxResults := 100
	if mr, ok := request.Params.Arguments["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	s.logger.Info("MCP progressive query", zap.String("question", question))

	req := types.ProgressiveQueryRequest{
		Question:   question,
		MaxResults: maxResults,
	}

	resp, err := s.queryService.ProgressiveQuery(ctx, req)
	if err != nil {
		s.logger.Error("progressive query failed", zap.Error(err))
		return nil, fmt.Errorf("query failed: %w", err)
	}

	resultText := formatProgressiveQueryResults(resp)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetDocument handles the tiersum_get_document tool
func (s *MCPServer) handleGetDocument(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docID, ok := request.Params.Arguments["document_id"].(string)
	if !ok || docID == "" {
		return nil, fmt.Errorf("document_id is required")
	}

	s.logger.Info("MCP get document", zap.String("document_id", docID))

	doc, err := s.docService.Get(ctx, docID)
	if err != nil {
		s.logger.Error("failed to get document", zap.String("id", docID), zap.Error(err))
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if doc == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Document not found: %s", docID)), nil
	}

	resultText := formatDocument(doc)
	return mcp.NewToolResultText(resultText), nil
}

// handleListTagGroups handles the tiersum_list_tag_groups tool
func (s *MCPServer) handleListTagGroups(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.Info("MCP list tag groups")

	if s.tagGroupService == nil {
		return nil, fmt.Errorf("tag grouping service not available")
	}

	groups, err := s.tagGroupService.GetL1Groups(ctx)
	if err != nil {
		s.logger.Error("failed to list groups", zap.Error(err))
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	resultText := formatTagGroups(groups)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetTagsByGroup handles the tiersum_get_tags_by_group tool
func (s *MCPServer) handleGetTagsByGroup(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	groupID, ok := request.Params.Arguments["group_id"].(string)
	if !ok || groupID == "" {
		return nil, fmt.Errorf("group_id is required")
	}

	s.logger.Info("MCP get tags by group", zap.String("group_id", groupID))

	if s.tagGroupService == nil {
		return nil, fmt.Errorf("tag grouping service not available")
	}

	tags, err := s.tagGroupService.GetL2TagsByGroup(ctx, groupID)
	if err != nil {
		s.logger.Error("failed to get tags", zap.Error(err))
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	resultText := formatTagsByGroup(groupID, tags)
	return mcp.NewToolResultText(resultText), nil
}

// handleTriggerTagGroup handles the tiersum_trigger_tag_grouping tool
func (s *MCPServer) handleTriggerTagGroup(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.Info("MCP trigger tag grouping")

	if s.tagGroupService == nil {
		return nil, fmt.Errorf("tag grouping service not available")
	}

	if err := s.tagGroupService.GroupTags(ctx); err != nil {
		s.logger.Error("failed to group tags", zap.Error(err))
		return nil, fmt.Errorf("failed to group tags: %w", err)
	}

	return mcp.NewToolResultText("Tag grouping completed successfully"), nil
}

// format functions
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

func formatProgressiveQueryResults(resp *types.ProgressiveQueryResponse) string {
	text := fmt.Sprintf("Query: %s\n\nProgressive Query Results:\n", resp.Question)

	// Show steps
	text += "\n=== Query Steps ===\n"
	for _, step := range resp.Steps {
		text += fmt.Sprintf("- %s: %v (took %dms)\n", step.Step, step.Output, step.Duration)
	}

	// Show results
	text += fmt.Sprintf("\n=== Results (%d items) ===\n", len(resp.Results))
	for i, item := range resp.Results {
		text += fmt.Sprintf("\n%d. %s (relevance: %.2f)\n", i+1, item.Title, item.Relevance)
		text += fmt.Sprintf("   Path: %s\n", item.Path)
		content := truncateString(item.Content, 300)
		text += fmt.Sprintf("   %s\n", content)
	}

	return text
}

func formatDocument(doc *types.Document) string {
	return fmt.Sprintf(
		"Document: %s\nID: %s\nFormat: %s\nTags: %v\nCreated: %s\n\nContent Preview:\n%s",
		doc.Title,
		doc.ID,
		doc.Format,
		doc.Tags,
		doc.CreatedAt.Format("2006-01-02 15:04:05"),
		truncateString(doc.Content, 500),
	)
}

func formatTagGroups(groups []types.TagGroup) string {
	if len(groups) == 0 {
		return "No tag groups found. Tags may not have been grouped yet."
	}

	text := fmt.Sprintf("Tag Groups (%d):\n\n", len(groups))
	for i, g := range groups {
		text += fmt.Sprintf("%d. %s\n", i+1, g.Name)
		text += fmt.Sprintf("   Description: %s\n", g.Description)
		text += fmt.Sprintf("   Tags (%d): %v\n\n", len(g.Tags), g.Tags)
	}
	return text
}

func formatTagsByGroup(groupID string, tags []types.Tag) string {
	if len(tags) == 0 {
		return fmt.Sprintf("No tags found in group %s", groupID)
	}

	text := fmt.Sprintf("Tags in Group (%d):\n\n", len(tags))
	for i, t := range tags {
		text += fmt.Sprintf("%d. %s (used in %d documents)\n", i+1, t.Name, t.DocumentCount)
	}
	return text
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
