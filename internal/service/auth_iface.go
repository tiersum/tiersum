package service

import (
	"context"
	"time"
)

// APIKeyPrincipal is the authenticated identity for /api/v1 and MCP (service track).
type APIKeyPrincipal struct {
	KeyID string
	Scope string
	Name  string
}

// BrowserPrincipal is the authenticated identity for /bff/v1 after session cookie is set.
type BrowserPrincipal struct {
	UserID    string
	Username  string
	Role      string
	SessionID string
}

// BootstrapResult is returned once when the system is first initialized.
type BootstrapResult struct {
	AdminUsername         string
	AdminAccessTokenPlain string
	InitialAPIKeyPlain    string
	InitialAPIKeyScope    string
	InitializedAt         time.Time
}

// CreatedSecretOnce holds a plaintext secret shown only at creation time.
type CreatedSecretOnce struct {
	Plaintext string
}

// UserSummary is a non-sensitive user row for admin lists.
type UserSummary struct {
	ID              string     `json:"id"`
	Username        string     `json:"username"`
	Role            string     `json:"role"`
	TokenExpiryMode string     `json:"token_expiry_mode"`
	MaxDevices      int        `json:"max_devices"`
	TokenValidUntil *time.Time `json:"token_valid_until,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// APIKeySummary is metadata for an API key (never includes key hash or plaintext).
type APIKeySummary struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Scope           string     `json:"scope"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedByUserID *string    `json:"created_by_user_id,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP      string     `json:"last_used_ip,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// BrowserDeviceSummary is one bound session/device for UI lists.
type BrowserDeviceSummary struct {
	SessionID   string    `json:"session_id"`
	DeviceAlias string    `json:"device_alias"`
	IPPrefix    string    `json:"ip_prefix"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// AdminBrowserDeviceSummary is a device row in the admin-wide device list (all users).
type AdminBrowserDeviceSummary struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	SessionID   string    `json:"session_id"`
	DeviceAlias string    `json:"device_alias"`
	IPPrefix    string    `json:"ip_prefix"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// FingerprintInput is submitted once at login (browser collects coarse signals).
type FingerprintInput struct {
	Timezone     string `json:"timezone"`
	ClientSignal string `json:"client_signal,omitempty"` // optional salt from front-end fingerprint helper
}

// IProgramAuth is the minimal surface for /api/v1 and MCP (service track).
type IProgramAuth interface {
	IsSystemInitialized(ctx context.Context) (bool, error)
	ValidateAPIKey(ctx context.Context, bearerToken string) (*APIKeyPrincipal, error)
	APIKeyMeetsScope(principal *APIKeyPrincipal, requiredScope string) bool
	RecordAPIKeyUse(ctx context.Context, keyID, method, path, clientIP string) error
}

// IAuthService covers program track plus bootstrap, browser session, and admin operations.
type IAuthService interface {
	IProgramAuth

	Bootstrap(ctx context.Context, adminUsername string) (*BootstrapResult, error)

	LoginWithAccessToken(ctx context.Context, accessTokenPlain string, fp FingerprintInput, remoteIP, userAgent string) (sessionCookiePlain string, err error)
	ValidateBrowserSession(ctx context.Context, sessionCookiePlain, remoteIP, userAgent string) (*BrowserPrincipal, error)
	LogoutSession(ctx context.Context, sessionCookiePlain string) error

	SlideTouchFromBrowserRequest(ctx context.Context, userID string) error

	CreateUser(ctx context.Context, actor *BrowserPrincipal, username, role string) (*CreatedSecretOnce, error)
	ResetUserAccessToken(ctx context.Context, actor *BrowserPrincipal, targetUserID string) (*CreatedSecretOnce, error)
	ListUsers(ctx context.Context, actor *BrowserPrincipal) ([]UserSummary, error)

	CreateAPIKey(ctx context.Context, actor *BrowserPrincipal, name, scope string, expiresAt *time.Time) (*CreatedSecretOnce, *APIKeySummary, error)
	RevokeAPIKey(ctx context.Context, actor *BrowserPrincipal, keyID string) error
	ListAPIKeys(ctx context.Context, actor *BrowserPrincipal) ([]APIKeySummary, error)

	ListOwnDevices(ctx context.Context, actor *BrowserPrincipal) ([]BrowserDeviceSummary, error)
	ListUserDevicesAdmin(ctx context.Context, actor *BrowserPrincipal, targetUserID string) ([]BrowserDeviceSummary, error)
	ListAllDevicesAdmin(ctx context.Context, actor *BrowserPrincipal) ([]AdminBrowserDeviceSummary, error)
	UpdateDeviceAlias(ctx context.Context, actor *BrowserPrincipal, sessionID, alias string) error
	RevokeDeviceSession(ctx context.Context, actor *BrowserPrincipal, sessionID string) error
	RevokeAllOwnSessions(ctx context.Context, actor *BrowserPrincipal) error

	APIKeyUsageCountsSince(ctx context.Context, actor *BrowserPrincipal, since time.Time) (map[string]int64, error)
}
