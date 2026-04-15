// Package service defines service-layer facades and internal contracts.
//
// types.go centralizes small service-layer types/structs and sentinel errors that are shared
// across facade interfaces and service implementations.
package service

import (
	"errors"
	"time"
)

// ---- Common service errors (used across API/Job and service implementations) ----

// ErrColdIndexUnavailable is returned when cold hybrid search is requested but no cold index is configured.
var ErrColdIndexUnavailable = errors.New("cold document index not available")

// ErrIngestValidation is returned when CreateDocument fails configurable policy checks (format, size, chunking).
var ErrIngestValidation = errors.New("ingest validation failed")

// ---- Auth errors (sentinel; mapped by API layer) ----

var (
	ErrAuthSystemNotInitialized = errors.New("system not initialized")
	ErrAuthAlreadyInitialized   = errors.New("system already initialized")
	ErrAuthInvalidAccessToken   = errors.New("invalid access token")
	ErrAuthInvalidDeviceToken   = errors.New("invalid device token")
	ErrAuthInvalidSession       = errors.New("invalid or expired session")
	ErrAuthDeviceLimit          = errors.New("device limit reached")
	ErrAuthForbidden            = errors.New("forbidden")
	ErrAuthInvalidAPIKey        = errors.New("invalid api key")
	ErrAuthAPIKeyRevoked        = errors.New("api key revoked")
	ErrAuthInsufficientScope    = errors.New("insufficient scope")
)

// ---- Auth DTOs shared by API layer and auth implementations ----

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

// DeviceTokenSummary is one persistent device-login credential (opaque cookie) metadata for UI lists.
// The plaintext token is never returned after initial issuance.
type DeviceTokenSummary struct {
	ID         string     `json:"id"`
	DeviceName string     `json:"device_name"`
	IPPrefix   string     `json:"ip_prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// PasskeySummary is one WebAuthn credential metadata for UI lists.
type PasskeySummary struct {
	ID         string     `json:"id"`
	DeviceName string     `json:"device_name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// PasskeyStatus summarizes whether a user has passkeys and whether this session is verified recently.
type PasskeyStatus struct {
	HasAny           bool       `json:"has_any"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	RequiredForAdmin bool       `json:"required_for_admin"`
}
