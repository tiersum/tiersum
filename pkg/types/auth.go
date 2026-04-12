package types

// Auth role for human-track users (browser).
const (
	AuthRoleAdmin = "admin"
	AuthRoleUser  = "user"
)

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
