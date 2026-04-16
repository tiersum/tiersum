package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tiersum/tiersum/pkg/types"
)

// BFFHumanRBAC enforces human-track role rules on /bff/v1 after BFFSessionMiddleware.
// Programmatic /api/v1 is unchanged (API key scopes only).
func BFFHumanRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := BrowserPrincipal(c)
		if p == nil {
			c.Next()
			return
		}
		role := strings.ToLower(strings.TrimSpace(p.Role))
		path := c.Request.URL.Path
		method := c.Request.Method

		// Ops / observability: admin only (same surface as monitoring UI + trace APIs).
		if strings.HasPrefix(path, "/bff/v1/monitoring") || strings.HasPrefix(path, "/bff/v1/traces") {
			if role != types.AuthRoleAdmin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":  "ADMIN_ROLE_REQUIRED",
					"error": "This endpoint requires an administrator role.",
				})
				return
			}
		}

		// Viewer: read-only in the browser; progressive query is treated as a read operation.
		if role == types.AuthRoleViewer {
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				c.Next()
				return
			}
			if method == http.MethodPost && path == "/bff/v1/query/progressive" {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":  "VIEWER_READ_ONLY",
				"error": "Viewer role is read-only. Ask an administrator to upgrade your account for ingest and management actions.",
			})
			return
		}

		c.Next()
	}
}
