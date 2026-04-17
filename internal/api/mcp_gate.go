package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/tiersum/tiersum/pkg/types"
)

// mcpProgramGate validates the API key carried in the MCP context against the same rules as /api/v1.
// The key is injected by the SSE context func in NewMCPServer (reads X-API-Key or Authorization header).
// logicalPath is recorded in audit (e.g. "POST /api/v1/documents").
func (s *MCPServer) mcpProgramGate(ctx context.Context, requiredScope, logicalPath string) (*mcp.CallToolResult, error) {
	if s.programAuth == nil {
		return nil, nil
	}
	raw := ""
	if v, ok := ctx.Value(mcpAPIKeyCtxValueKey).(string); ok {
		raw = strings.TrimSpace(v)
	}
	if raw == "" {
		return mcpJSONResult(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
	}
	_, status, body := checkProgramAuth(ctx, s.programAuth, raw, requiredScope, "MCP", logicalPath, "mcp")
	if status != 0 {
		return mcpJSONResult(status, body)
	}
	return nil, nil
}

// mcpReadGate is a shortcut for read-scope tools.
func (s *MCPServer) mcpReadGate(ctx context.Context, logicalPath string) (*mcp.CallToolResult, error) {
	return s.mcpProgramGate(ctx, types.AuthScopeRead, logicalPath)
}

func (s *MCPServer) mcpWriteGate(ctx context.Context, logicalPath string) (*mcp.CallToolResult, error) {
	return s.mcpProgramGate(ctx, types.AuthScopeWrite, logicalPath)
}
