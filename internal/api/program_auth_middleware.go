package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

const (
	ginKeyAPIKeyPrincipal = "apiKeyPrincipal"
)

// APIKeyPrincipal returns the service-track principal set by ProgramAuthMiddleware, or nil.
func APIKeyPrincipal(c *gin.Context) *service.APIKeyPrincipal {
	v, ok := c.Get(ginKeyAPIKeyPrincipal)
	if !ok {
		return nil
	}
	p, _ := v.(*service.APIKeyPrincipal)
	return p
}

// checkProgramAuth validates system initialization, API key, and required scope.
// It is shared between REST middleware and MCP gate so that both paths enforce the same rules.
func checkProgramAuth(
	ctx context.Context,
	auth service.IProgramAuth,
	rawKey, requiredScope, method, path, clientIP string,
) (*service.APIKeyPrincipal, int, gin.H) {
	init, err := auth.IsSystemInitialized(ctx)
	if err != nil {
		return nil, http.StatusInternalServerError, gin.H{"error": "auth_state_unavailable"}
	}
	if !init {
		return nil, http.StatusForbidden, gin.H{"code": "SYSTEM_NOT_INITIALIZED"}
	}

	principal, err := auth.ValidateAPIKey(ctx, rawKey)
	if err != nil {
		switch err {
		case service.ErrAuthAPIKeyRevoked:
			return nil, http.StatusForbidden, gin.H{"error": "key_revoked", "contact_admin": true}
		case service.ErrAuthInvalidAPIKey:
			return nil, http.StatusUnauthorized, gin.H{"error": "invalid_key"}
		default:
			return nil, http.StatusUnauthorized, gin.H{"error": "invalid_key"}
		}
	}

	if !auth.APIKeyMeetsScope(principal, requiredScope) {
		return nil, http.StatusForbidden, gin.H{
			"error":    "insufficient_scope",
			"required": requiredScope,
		}
	}

	_ = auth.RecordAPIKeyUse(ctx, principal.KeyID, method, path, clientIP)
	return principal, 0, nil
}

// ProgramAuthMiddleware enforces system initialization and API key + scope on /api/v1.
// When programAuth is nil, the middleware is a no-op (should not happen in production wiring).
func ProgramAuthMiddleware(programAuth service.IProgramAuth) gin.HandlerFunc {
	if programAuth == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if raw == "" {
			authz := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				raw = strings.TrimSpace(authz[7:])
			}
		}

		required := apiRouteRequiredScope(c.Request.Method, c.Request.URL.Path)
		principal, status, body := checkProgramAuth(
			c.Request.Context(), programAuth, raw, required,
			c.Request.Method, c.Request.URL.Path, c.ClientIP(),
		)
		if status != 0 {
			c.AbortWithStatusJSON(status, body)
			return
		}

		c.Set(ginKeyAPIKeyPrincipal, principal)
		c.Next()
	}
}

func apiRouteRequiredScope(method, absPath string) string {
	p := strings.TrimPrefix(absPath, "/api/v1")
	p = strings.TrimPrefix(p, "/")
	m := strings.ToUpper(method)

	if m == "POST" && p == "documents" {
		return types.AuthScopeWrite
	}
	if m == "POST" && p == "topics/regroup" {
		return types.AuthScopeWrite
	}
	return types.AuthScopeRead
}
