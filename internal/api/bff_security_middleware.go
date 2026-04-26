package api

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("auth.browser.csrf.enabled", true)
	viper.SetDefault("auth.browser.csrf.require_origin_or_referer", true)
	viper.SetDefault("auth.browser.csrf.allowed_origins", []string{})
}

// BFFSameOriginMiddleware protects browser-session endpoints from cross-site requests.
// For unsafe methods, it requires Origin or Referer to match the current host, unless explicitly allowlisted.
func BFFSameOriginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !viper.GetBool("auth.browser.csrf.enabled") {
			c.Next()
			return
		}
		m := strings.ToUpper(strings.TrimSpace(c.Request.Method))
		switch m {
		case "GET", "HEAD", "OPTIONS":
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		ref := strings.TrimSpace(c.GetHeader("Referer"))
		if origin == "" && ref == "" {
			if viper.GetBool("auth.browser.csrf.require_origin_or_referer") {
				c.AbortWithStatusJSON(403, gin.H{"error": "csrf_blocked"})
				return
			}
			c.Next()
			return
		}

		host := strings.ToLower(strings.TrimSpace(c.Request.Host))
		host = stripPort(host)

		if origin != "" && originAllowed(origin, host) {
			c.Next()
			return
		}
		if ref != "" && refererAllowed(ref, host) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(403, gin.H{"error": "csrf_blocked"})
	}
}

func stripPort(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}

func originAllowed(origin string, reqHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u == nil {
		return false
	}
	oh := stripPort(strings.ToLower(strings.TrimSpace(u.Host)))
	if oh == "" {
		return false
	}
	if oh == reqHost {
		return true
	}
	for _, raw := range viper.GetStringSlice("auth.browser.csrf.allowed_origins") {
		if strings.EqualFold(stripPort(strings.TrimSpace(raw)), oh) {
			return true
		}
	}
	return false
}

func refererAllowed(referer string, reqHost string) bool {
	u, err := url.Parse(referer)
	if err != nil || u == nil {
		return false
	}
	rh := stripPort(strings.ToLower(strings.TrimSpace(u.Host)))
	if rh == "" {
		return false
	}
	if rh == reqHost {
		return true
	}
	for _, raw := range viper.GetStringSlice("auth.browser.csrf.allowed_origins") {
		if strings.EqualFold(stripPort(strings.TrimSpace(raw)), rh) {
			return true
		}
	}
	return false
}
