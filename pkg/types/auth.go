package types

import "strings"

// Auth role for human-track users (browser).
const (
	AuthRoleAdmin  = "admin"
	AuthRoleUser   = "user"
	AuthRoleViewer = "viewer"
)

// IsValidHumanBrowserRole reports whether role is a known browser-track role (case-insensitive).
func IsValidHumanBrowserRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case AuthRoleAdmin, AuthRoleUser, AuthRoleViewer:
		return true
	default:
		return false
	}
}

// API key scopes for service track (rank: read < write < admin).
const (
	AuthScopeRead  = "read"
	AuthScopeWrite = "write"
	AuthScopeAdmin = "admin"
)

// Token expiry modes for human-track access tokens.
const (
	AuthTokenExpirySlide = "slide"
	AuthTokenExpiryNever = "never"
)
