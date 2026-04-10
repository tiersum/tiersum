package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed web/*
var webFS embed.FS

// StaticFileServer returns a gin handler for serving embedded static files
func StaticFileServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		
		// Skip API and health routes
		if strings.HasPrefix(path, "/api/") || 
		   strings.HasPrefix(path, "/health") || 
		   strings.HasPrefix(path, "/mcp/") {
			c.Next()
			return
		}

		// Clean path
		if path == "/" {
			path = "/index.html"
		}
		
		// Try to serve file from embed
		filePath := "web" + path
		data, err := webFS.ReadFile(filePath)
		if err != nil {
			// If file not found, try index.html for SPA routes
			data, err = webFS.ReadFile("web/index.html")
			if err != nil {
				c.Next()
				return
			}
			filePath = "web/index.html"
		}

		// Set content type based on extension
		contentType := getContentType(filePath)
		c.Header("Content-Type", contentType)
		c.Data(http.StatusOK, contentType, data)
		c.Abort()
	}
}

func getContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// WebFS returns the embedded filesystem
func WebFS() fs.FS {
	return webFS
}
