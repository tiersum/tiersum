package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tiersum/tiersum/internal/service"
)

// BFFRequireAdminPasskey aborts admin JSON calls unless the browser session recently completed a passkey assertion.
// Admins without any registered passkeys are allowed through so the first passkey can be enrolled from /settings.
func BFFRequireAdminPasskey(auth service.IAuthService) gin.HandlerFunc {
	if auth == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		// BFFRequireAdmin runs before this middleware on /bff/v1/admin/*.
		p := BrowserPrincipal(c)
		if p == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		st, err := auth.PasskeyStatus(c.Request.Context(), p)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "passkey_status_unavailable"})
			return
		}
		if !st.RequiredForAdmin {
			c.Next()
			return
		}
		if !st.HasAny {
			c.Next()
			return
		}
		if st.VerifiedAt == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "passkey_required"})
			return
		}
		c.Next()
	}
}
