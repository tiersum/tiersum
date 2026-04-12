package api

import (
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

// ProgramAuthMiddleware enforces system initialization and API key + scope on /api/v1.
// When programAuth is nil, the middleware is a no-op (should not happen in production wiring).
func ProgramAuthMiddleware(programAuth service.IProgramAuth) gin.HandlerFunc {
	if programAuth == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		init, err := programAuth.IsSystemInitialized(ctx)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth_state_unavailable"})
			return
		}
		if !init {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "SYSTEM_NOT_INITIALIZED"})
			return
		}

		raw := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if raw == "" {
			authz := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				raw = strings.TrimSpace(authz[7:])
			}
		}
		principal, err := programAuth.ValidateAPIKey(ctx, raw)
		if err != nil {
			switch err {
			case service.ErrAuthAPIKeyRevoked:
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "key_revoked", "contact_admin": true})
			case service.ErrAuthInvalidAPIKey:
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
			default:
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
			}
			return
		}

		required := apiRouteRequiredScope(c.Request.Method, c.Request.URL.Path)
		if !programAuth.APIKeyMeetsScope(principal, required) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "insufficient_scope",
				"required": required,
			})
			return
		}

		_ = programAuth.RecordAPIKeyUse(ctx, principal.KeyID, c.Request.Method, c.Request.URL.Path, c.ClientIP())
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
	if m == "POST" && p == "tags/group" {
		return types.AuthScopeWrite
	}
	return types.AuthScopeRead
}
