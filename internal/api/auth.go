package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BFFAuth is reserved for browser-facing /bff/* routes (sessions, cookies, OIDC, etc.).
// It is currently a no-op so the embedded UI can call the BFF without sharing the programmatic API key.
func BFFAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// APIKeyAuth returns middleware that enforces X-API-Key or Authorization: Bearer when apiKey is non-empty.
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	if apiKey == "" {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		if c.GetHeader("X-API-Key") == apiKey {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == apiKey {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}
