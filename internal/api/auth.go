package api

import (
	"github.com/gin-gonic/gin"
)

// BFFAuth is reserved for browser-facing /bff/* routes; the real gate is BFFSessionMiddleware
// wired from cmd/main.go together with public auth routes on /bff/v1.
func BFFAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
