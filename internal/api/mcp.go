// Package api implements API layer
// Includes REST API and MCP protocol handlers
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/pkg/types"
)

// MCPServer handles MCP protocol. Tool names and payloads mirror REST /api/v1 (see RegisterRoutes).
type MCPServer struct {
	api    *Handler
	logger *zap.Logger
	mcp    *mcpserver.MCPServer
	sse    *mcpserver.SSEServer
}

// NewMCPServer creates a new MCP server wired to the same Handler as REST.
func NewMCPServer(restAPI *Handler, logger *zap.Logger) *MCPServer {
	s := &MCPServer{
		api:    restAPI,
		logger: logger,
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

func (s *MCPServer) registerTools() {
	const descPrefix = "Same semantics as "

	s.mcp.AddTool(mcp.NewTool("api_v1_documents_post",
		mcp.WithDescription(descPrefix+"POST /api/v1/documents — ingest document (JSON body)."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Document title")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Document body")),
		mcp.WithString("format", mcp.Required(), mcp.Description("markdown or md"), mcp.Enum("markdown", "md")),
		mcp.WithString("tags", mcp.Description("Optional: JSON array string e.g. [\"a\",\"b\"]")),
		mcp.WithString("summary", mcp.Description("Optional pre-built document summary")),
		mcp.WithString("chapters", mcp.Description("Optional JSON array of ChapterInfo")),
		mcp.WithString("ingest_mode", mcp.Description("auto (default) | hot | cold — hot/cold tier on ingest"), mcp.Enum("auto", "hot", "cold")),
		mcp.WithBoolean("force_hot", mcp.Description("Deprecated: use ingest_mode=hot")),
		mcp.WithString("embedding", mcp.Description("Optional JSON array of numbers (float32 embedding)")),
	), s.handleAPIv1DocumentsPost)

	s.mcp.AddTool(mcp.NewTool("api_v1_documents_list",
		mcp.WithDescription(descPrefix+"GET /api/v1/documents"),
	), s.handleAPIv1DocumentsList)

	s.mcp.AddTool(mcp.NewTool("api_v1_documents_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/documents/:id"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Document id")),
	), s.handleAPIv1DocumentsGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_documents_chapters_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/documents/:id/chapters"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Document id")),
	), s.handleAPIv1DocumentsChaptersGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_documents_summaries_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/documents/:id/summaries"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Document id")),
	), s.handleAPIv1DocumentsSummariesGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_query_progressive_post",
		mcp.WithDescription(descPrefix+"POST /api/v1/query/progressive"),
		mcp.WithString("question", mcp.Required(), mcp.Description("User question")),
		mcp.WithNumber("max_results", mcp.Description("1–100; omit or 0 for default 100")),
	), s.handleAPIv1QueryProgressivePost)

	s.mcp.AddTool(mcp.NewTool("api_v1_tags_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/tags — optional group_ids (comma-separated) and max_results"),
		mcp.WithString("group_ids", mcp.Description("Comma-separated group ids; omit for all tags")),
		mcp.WithNumber("max_results", mcp.Description("Cap; with group_ids defaults to 100, max 10000")),
	), s.handleAPIv1TagsGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_tags_groups_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/tags/groups"),
	), s.handleAPIv1TagsGroupsGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_tags_group_post",
		mcp.WithDescription(descPrefix+"POST /api/v1/tags/group — trigger tag grouping"),
	), s.handleAPIv1TagsGroupPost)

	s.mcp.AddTool(mcp.NewTool("api_v1_hot_doc_summaries_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/hot/doc_summaries — tags comma-separated, max_results"),
		mcp.WithString("tags", mcp.Required(), mcp.Description("Comma-separated tags (same as query param `tags`)")),
		mcp.WithNumber("max_results", mcp.Description("Default 1000, max 10000")),
	), s.handleAPIv1HotDocSummariesGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_hot_doc_chapters_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/hot/doc_chapters — doc_ids comma-separated"),
		mcp.WithString("doc_ids", mcp.Required(), mcp.Description("Comma-separated document ids")),
		mcp.WithNumber("max_results", mcp.Description("Max number of doc ids; default 100, max 500")),
	), s.handleAPIv1HotDocChaptersGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_hot_doc_source_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/hot/doc_source — chapter_paths comma-separated"),
		mcp.WithString("chapter_paths", mcp.Required(), mcp.Description("Comma-separated chapter paths")),
		mcp.WithNumber("max_results", mcp.Description("Default 100, max 2000")),
	), s.handleAPIv1HotDocSourceGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_cold_doc_source_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/cold/doc_source — q comma-separated terms; hybrid search returns cold chapters (path + full text)"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Comma-separated keywords (same as query param `q`)")),
		mcp.WithNumber("max_results", mcp.Description("Default 100, max 500")),
	), s.handleAPIv1ColdDocSourceGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_quota_get",
		mcp.WithDescription(descPrefix+"GET /api/v1/quota"),
	), s.handleAPIv1QuotaGet)

	s.mcp.AddTool(mcp.NewTool("api_v1_metrics_get",
		mcp.WithDescription("Same as GET /api/v1/metrics — Prometheus text exposition format in the tool response body."),
	), s.handleAPIv1MetricsGet)
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

// argStringList accepts a comma-separated string, JSON array string, or JSON array in arguments (same values as REST query lists).
func argStringList(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		return parseCommaSeparated(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func optionalMaxResultsQueryString(args map[string]any, key string) string {
	if f, ok := argFloat(args, key); ok && int(f) > 0 {
		return strconv.Itoa(int(f))
	}
	return ""
}

func mcpJSONResult(status int, body any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	isErr := status >= http.StatusBadRequest
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: string(b)},
		},
		IsError: isErr,
	}, nil
}

func (s *MCPServer) handleAPIv1DocumentsPost(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	title := strings.TrimSpace(argString(args, "title"))
	content := strings.TrimSpace(argString(args, "content"))
	format := strings.TrimSpace(argString(args, "format"))
	if title == "" || content == "" || format == "" {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "title, content, and format are required (format: markdown or md)"})
	}
	if format != "markdown" && format != "md" {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "format must be markdown or md"})
	}
	req := types.CreateDocumentRequest{
		Title:    title,
		Content:  content,
		Format:   format,
		Tags:     parseTagsArg(args["tags"]),
		Chapters: parseChaptersArg(args["chapters"]),
	}
	if summary := argString(args, "summary"); summary != "" {
		req.Summary = summary
	}
	if im := strings.TrimSpace(argString(args, "ingest_mode")); im != "" {
		req.IngestMode = im
	}
	if forceHot, ok := args["force_hot"].(bool); ok {
		req.ForceHot = forceHot
	}
	if emb := parseEmbeddingArg(args["embedding"]); len(emb) > 0 {
		req.Embedding = emb
	}
	status, body := s.api.ExecuteIngestDocument(ctx, req)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1DocumentsList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = toolArgs(request)
	status, body := s.api.ExecuteListDocuments(ctx)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1DocumentsGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := strings.TrimSpace(argString(toolArgs(request), "id"))
	if id == "" {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "id is required"})
	}
	status, body := s.api.ExecuteGetDocument(ctx, id)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1DocumentsChaptersGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := strings.TrimSpace(argString(toolArgs(request), "id"))
	if id == "" {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "id is required"})
	}
	status, body := s.api.ExecuteGetDocumentChapters(ctx, id)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1DocumentsSummariesGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := strings.TrimSpace(argString(toolArgs(request), "id"))
	if id == "" {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "id is required"})
	}
	status, body := s.api.ExecuteGetDocumentSummaries(ctx, id)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1QueryProgressivePost(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	q := strings.TrimSpace(argString(args, "question"))
	if q == "" {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "Key: 'ProgressiveQueryRequest.Question' Error:Field validation for 'Question' failed on the 'required' tag"})
	}
	mr := 0
	if f, ok := argFloat(args, "max_results"); ok {
		mr = int(f)
	}
	if mr == 0 {
		mr = 100
	}
	if mr < 1 || mr > 100 {
		return mcpJSONResult(http.StatusBadRequest, gin.H{"error": "Key: 'ProgressiveQueryRequest.MaxResults' Error:Field validation for 'MaxResults' failed on the 'max' tag"})
	}
	req := types.ProgressiveQueryRequest{Question: q, MaxResults: mr}
	status, body := s.api.ExecuteProgressiveQuery(ctx, req)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1TagsGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	groupIDs := argStringList(args, "group_ids")
	maxRaw := optionalMaxResultsQueryString(args, "max_results")
	status, body := s.api.ExecuteListTags(ctx, groupIDs, maxRaw)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1TagsGroupsGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = toolArgs(request)
	status, body := s.api.ExecuteListTagGroups(ctx)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1TagsGroupPost(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = toolArgs(request)
	status, body := s.api.ExecuteTriggerTagGroup(ctx)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1HotDocSummariesGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	tags := argStringList(args, "tags")
	maxRaw := optionalMaxResultsQueryString(args, "max_results")
	status, body := s.api.ExecuteHotDocSummaries(ctx, tags, maxRaw)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1HotDocChaptersGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	docIDs := argStringList(args, "doc_ids")
	maxRaw := optionalMaxResultsQueryString(args, "max_results")
	status, body := s.api.ExecuteHotDocChapters(ctx, docIDs, maxRaw)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1HotDocSourceGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	paths := argStringList(args, "chapter_paths")
	maxRaw := optionalMaxResultsQueryString(args, "max_results")
	status, body := s.api.ExecuteHotDocSource(ctx, paths, maxRaw)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1ColdDocSourceGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := toolArgs(request)
	terms := argStringList(args, "q")
	maxRaw := optionalMaxResultsQueryString(args, "max_results")
	status, body := s.api.ExecuteColdDocSource(ctx, terms, maxRaw)
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1QuotaGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = toolArgs(request)
	status, body := s.api.ExecuteGetQuota()
	return mcpJSONResult(status, body)
}

func (s *MCPServer) handleAPIv1MetricsGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = toolArgs(request)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	promhttp.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("metrics endpoint returned HTTP %d", rec.Code)), nil
	}
	return mcp.NewToolResultText(rec.Body.String()), nil
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

func parseEmbeddingArg(raw any) []float32 {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var nums []float64
		if err := json.Unmarshal([]byte(v), &nums); err != nil {
			return nil
		}
		out := make([]float32, len(nums))
		for i, x := range nums {
			out[i] = float32(x)
		}
		return out
	case []any:
		out := make([]float32, 0, len(v))
		for _, x := range v {
			switch n := x.(type) {
			case float64:
				out = append(out, float32(n))
			case float32:
				out = append(out, n)
			case int:
				out = append(out, float32(n))
			}
		}
		return out
	default:
		return nil
	}
}
