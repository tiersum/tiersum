// Package api implements API layer
// Includes REST API and MCP protocol handlers
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// MCPServer handles MCP protocol
type MCPServer struct {
	docService      service.IDocumentService
	queryService    service.IQueryService
	tagGroupService service.ITagGroupService
	logger          *zap.Logger
	mcp             *mcpserver.MCPServer
	sse             *mcpserver.SSEServer
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

	s.mcp = mcpserver.NewMCPServer(
		"tiersum",
		"2.0.0",
		mcpserver.WithResourceCapabilities(true, true),
	)

	s.registerTools()

	baseURL := strings.TrimSpace(viper.GetString("mcp.base_url"))
	opts := []mcpserver.SSEOption{
		mcpserver.WithStaticBasePath("/mcp"),
		mcpserver.WithSSEEndpoint("/sse"),
		mcpserver.WithMessageEndpoint("/message"),
	}
	if baseURL != "" {
		opts = append(opts,
			mcpserver.WithBaseURL(strings.TrimSuffix(baseURL, "/")),
			mcpserver.WithUseFullURLForMessageEndpoint(true),
		)
	} else {
		opts = append(opts, mcpserver.WithUseFullURLForMessageEndpoint(false))
	}
	s.sse = mcpserver.NewSSEServer(s.mcp, opts...)

	return s
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() {
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

	getDocTool := mcp.NewTool("tiersum_get_document",
		mcp.WithDescription("Retrieve a document by ID"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID to retrieve"),
		),
	)
	s.mcp.AddTool(getDocTool, s.handleGetDocument)

	listGroupsTool := mcp.NewTool("tiersum_list_tag_groups",
		mcp.WithDescription("List all tag groups (Level 1 categories)"),
	)
	s.mcp.AddTool(listGroupsTool, s.handleListTagGroups)

	getTagsByGroupTool := mcp.NewTool("tiersum_get_tags_by_group",
		mcp.WithDescription("Get all tags (Level 2) belonging to a specific group"),
		mcp.WithString("group_id",
			mcp.Required(),
			mcp.Description("The group ID"),
		),
	)
	s.mcp.AddTool(getTagsByGroupTool, s.handleGetTagsByGroup)

	triggerGroupingTool := mcp.NewTool("tiersum_trigger_tag_grouping",
		mcp.WithDescription("Manually trigger tag grouping (normally runs automatically every 30 minutes)"),
	)
	s.mcp.AddTool(triggerGroupingTool, s.handleTriggerTagGroup)

	ingestDocTool := mcp.NewTool("tiersum_ingest_document",
		mcp.WithDescription("Ingest a document into the knowledge base. Supports pre-built summaries from external agents."),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Document title"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Document content"),
		),
		mcp.WithString("format",
			mcp.Description("Document format: markdown or md (default: markdown)"),
			mcp.Enum("markdown", "md"),
		),
		mcp.WithString("tags",
			mcp.Description("Optional tags as JSON array string, e.g. [\"tag1\",\"tag2\"], or provided as a JSON array by the client"),
		),
		mcp.WithString("summary",
			mcp.Description("Optional pre-built document summary (from external agent)"),
		),
		mcp.WithString("chapters",
			mcp.Description("Optional JSON array of chapters: [{\"title\":\"...\",\"summary\":\"...\",\"content\":\"...\"}]"),
		),
		mcp.WithBoolean("force_hot",
			mcp.Description("Force hot processing regardless of quota"),
		),
	)
	s.mcp.AddTool(ingestDocTool, s.handleIngestDocument)
}

// GetMCPServer returns the underlying MCP server for SSE handling
func (s *MCPServer) GetMCPServer() *mcpserver.MCPServer {
	return s.mcp
}

// SSEHTTP returns the SSE stream handler for GET /mcp/sse
func (s *MCPServer) SSEHTTP() http.Handler {
	return s.sse.SSEHandler()
}

// MCPMessageHTTP returns the JSON-RPC message handler for POST /mcp/message
func (s *MCPServer) MCPMessageHTTP() http.Handler {
	return s.sse.MessageHandler()
}

// SSEHandler returns a Gin handler for the MCP SSE endpoint
func (s *MCPServer) SSEHandler() gin.HandlerFunc {
	h := s.SSEHTTP()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// MessageHandler returns a Gin handler for the MCP message endpoint
func (s *MCPServer) MessageHandler() gin.HandlerFunc {
	h := s.MCPMessageHTTP()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func toolArgs(request mcp.CallToolRequest) map[string]any {
	args := request.GetArguments()
	if args == nil {
		return map[string]any{}
	}
	return args
}

func argString(args map[string]any, key string) string {
	v, ok := args[key].(string)
	if !ok {
		return ""
	}
	return v
}

func argFloat(args map[string]any, key string) (float64, bool) {
	switch v := args[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// handleQuery handles the tiersum_query tool
func (s *MCPServer) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	question := argString(args, "question")
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	depthStr := argString(args, "depth")
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
	args := toolArgs(request)
	question := argString(args, "question")
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	maxResults := 100
	if f, ok := argFloat(args, "max_results"); ok {
		maxResults = int(f)
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
	args := toolArgs(request)
	docID := argString(args, "document_id")
	if docID == "" {
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
	_ = toolArgs(request)
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
	args := toolArgs(request)
	groupID := argString(args, "group_id")
	if groupID == "" {
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
	_ = toolArgs(request)
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

// handleIngestDocument handles the tiersum_ingest_document tool
func (s *MCPServer) handleIngestDocument(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	title := argString(args, "title")
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	content := argString(args, "content")
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	format := "markdown"
	if f := argString(args, "format"); f != "" {
		format = f
	}

	req := types.CreateDocumentRequest{
		Title:    title,
		Content:  content,
		Format:   format,
		Tags:     []string{},
		Chapters: []types.ChapterInfo{},
	}

	req.Tags = append(req.Tags, parseTagsArg(args["tags"])...)

	if summary := argString(args, "summary"); summary != "" {
		req.Summary = summary
	}

	if ch := parseChaptersArg(args["chapters"]); len(ch) > 0 {
		req.Chapters = ch
	}

	if forceHot, ok := args["force_hot"].(bool); ok {
		req.ForceHot = forceHot
	}

	s.logger.Info("MCP ingest document",
		zap.String("title", title),
		zap.Int("tags", len(req.Tags)),
		zap.Int("chapters", len(req.Chapters)),
		zap.Bool("has_summary", req.Summary != ""),
		zap.Bool("force_hot", req.ForceHot))

	resp, err := s.docService.Ingest(ctx, req)
	if err != nil {
		s.logger.Error("failed to ingest document", zap.Error(err))
		return nil, fmt.Errorf("failed to ingest document: %w", err)
	}

	resultText := fmt.Sprintf(
		"Document ingested successfully:\nID: %s\nTitle: %s\nStatus: %s\nTags: %v",
		resp.ID,
		resp.Title,
		resp.Status,
		resp.Tags,
	)

	return mcp.NewToolResultText(resultText), nil
}

func parseTagsArg(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case []any:
		var out []string
		for _, t := range v {
			if s, ok := t.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var tags []string
		if err := json.Unmarshal([]byte(v), &tags); err != nil {
			return nil
		}
		return tags
	default:
		return nil
	}
}

func parseChaptersArg(raw any) []types.ChapterInfo {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var ch []types.ChapterInfo
		if err := json.Unmarshal([]byte(v), &ch); err != nil {
			return nil
		}
		return ch
	case []any:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var ch []types.ChapterInfo
		if err := json.Unmarshal(b, &ch); err != nil {
			return nil
		}
		return ch
	default:
		return nil
	}
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

	text += "\n=== Query Steps ===\n"
	for _, step := range resp.Steps {
		text += fmt.Sprintf("- %s: %v (took %dms)\n", step.Step, step.Output, step.Duration)
	}

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
