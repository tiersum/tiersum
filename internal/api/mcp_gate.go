package api

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// mcpProgramGate validates TIERSUM_API_KEY (or mcp.api_key) against the same rules as /api/v1.
// logicalPath is recorded in audit (e.g. "POST /api/v1/documents").
func (s *MCPServer) mcpProgramGate(ctx context.Context, requiredScope, logicalPath string) (*mcp.CallToolResult, error) {
	if s.programAuth == nil {
		return nil, nil
	}
	init, err := s.programAuth.IsSystemInitialized(ctx)
	if err != nil {
		return mcpJSONResult(http.StatusInternalServerError, gin.H{"error": "auth_state_unavailable"})
	}
	if !init {
		return mcpJSONResult(http.StatusForbidden, gin.H{"code": "SYSTEM_NOT_INITIALIZED"})
	}
	raw := strings.TrimSpace(os.Getenv("TIERSUM_API_KEY"))
	if raw == "" {
		raw = strings.TrimSpace(viper.GetString("mcp.api_key"))
	}
	if raw == "" {
		return mcpJSONResult(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
	}
	principal, err := s.programAuth.ValidateAPIKey(ctx, raw)
	if err != nil {
		switch err {
		case service.ErrAuthAPIKeyRevoked:
			return mcpJSONResult(http.StatusForbidden, gin.H{"error": "key_revoked", "contact_admin": true})
		default:
			return mcpJSONResult(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
		}
	}
	if !s.programAuth.APIKeyMeetsScope(principal, requiredScope) {
		return mcpJSONResult(http.StatusForbidden, gin.H{"error": "insufficient_scope", "required": requiredScope})
	}
	_ = s.programAuth.RecordAPIKeyUse(ctx, principal.KeyID, "MCP", logicalPath, "mcp")
	return nil, nil
}

// mcpReadGate is a shortcut for read-scope tools.
func (s *MCPServer) mcpReadGate(ctx context.Context, logicalPath string) (*mcp.CallToolResult, error) {
	return s.mcpProgramGate(ctx, types.AuthScopeRead, logicalPath)
}

func (s *MCPServer) mcpWriteGate(ctx context.Context, logicalPath string) (*mcp.CallToolResult, error) {
	return s.mcpProgramGate(ctx, types.AuthScopeWrite, logicalPath)
}
