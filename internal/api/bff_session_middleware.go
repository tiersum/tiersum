package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

const (
	sessionCookieName      = "tiersum_session"
	deviceCookieName       = "tiersum_device"
	ginKeyBrowserPrincipal = "browserPrincipal"
	bffPublicSystemStatus  = "/bff/v1/system/status"
	bffPublicBootstrap     = "/bff/v1/system/bootstrap"
	bffPublicLogin         = "/bff/v1/auth/login"
	bffPublicDeviceLogin   = "/bff/v1/auth/device_login"
	bffPublicLogout        = "/bff/v1/auth/logout"
)

// bffV1PublicPaths is the single source of truth for paths that skip session middleware
// (must stay aligned with AuthBFFHandler.RegisterPublicRoutes).
var bffV1PublicPaths = []string{
	bffPublicSystemStatus,
	bffPublicBootstrap,
	bffPublicLogin,
	bffPublicDeviceLogin,
	bffPublicLogout,
}

var bffPublicPathLookup map[string]struct{}

func init() {
	bffPublicPathLookup = make(map[string]struct{}, len(bffV1PublicPaths))
	for _, p := range bffV1PublicPaths {
		bffPublicPathLookup[p] = struct{}{}
	}
}

// BFFV1PublicPaths returns exact /bff/v1 paths that skip BFFSessionMiddleware (browser auth bootstrap and login).
func BFFV1PublicPaths() []string {
	out := make([]string, len(bffV1PublicPaths))
	copy(out, bffV1PublicPaths)
	return out
}

// BrowserPrincipal returns the human-track principal set by BFFSessionMiddleware, or nil.
func BrowserPrincipal(c *gin.Context) *service.BrowserPrincipal {
	v, ok := c.Get(ginKeyBrowserPrincipal)
	if !ok {
		return nil
	}
	p, _ := v.(*service.BrowserPrincipal)
	return p
}

func isBFFPublicPath(path string) bool {
	_, ok := bffPublicPathLookup[path]
	return ok
}

// BFFSessionMiddleware enforces browser session cookie on /bff/v1 except small public auth paths.
func BFFSessionMiddleware(auth service.IBFFSessionMiddlewareAuth) gin.HandlerFunc {
	if auth == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		if isBFFPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		init, err := auth.IsSystemInitialized(ctx)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth_state_unavailable"})
			return
		}
		if !init {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "SYSTEM_NOT_INITIALIZED"})
			return
		}

		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		p, err := auth.ValidateBrowserSession(ctx, cookie, c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set(ginKeyBrowserPrincipal, p)
		c.Next()
	}
}

// BFFRequireAdmin aborts with 403 unless the browser principal is an admin.
func BFFRequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := BrowserPrincipal(c)
		if p == nil || p.Role != types.AuthRoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
