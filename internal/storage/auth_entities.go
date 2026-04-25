// Package storage — auth-related row types (dual-track: browser + API keys).
package storage

import "time"

// SystemAuthState is the singleton bootstrap row (id always 1).
type SystemAuthState struct {
	InitializedAt *time.Time
}

// AuthUser is a human-track user (access token stored as hash only).
type AuthUser struct {
	ID              string
	Username        string
	Role            string // admin | user | viewer
	AccessTokenHash string // SHA256 hex of plaintext access token
	TokenExpiryMode string // slide | never
	MaxDevices      int
	TokenValidUntil *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// BrowserSession is a bound browser device + HTTP session cookie.
type BrowserSession struct {
	ID               string
	UserID           string
	SessionTokenHash string
	FingerprintHash  string
	IPPrefix         string
	UserAgentNorm    string
	Timezone         string
	DeviceAlias      string
	ExpiresAt        time.Time
	LastSeenAt       time.Time
	CreatedAt        time.Time
}

// BrowserSessionAdminListRow is a session joined with the owning username for admin-only lists.
type BrowserSessionAdminListRow struct {
	BrowserSession
	Username string
}

// DeviceToken is a persistent browser "keep me signed in" credential (plaintext stored only in cookie).
type DeviceToken struct {
	ID            string
	UserID        string
	TokenHash     string
	DeviceName    string
	IPPrefix      string
	UserAgentNorm string
	LastUsedAt    *time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
}

// PasskeyCredential is one WebAuthn credential registered for a user.
// Stored fields are base64url strings for portability (no driver-specific blobs).
type PasskeyCredential struct {
	ID              string
	UserID          string
	CredentialIDB64 string
	PublicKeyB64    string
	SignCount       int64
	DeviceName      string
	LastUsedAt      *time.Time
	CreatedAt       time.Time
}

// PasskeySessionVerification records that a browser session recently completed a passkey assertion.
type PasskeySessionVerification struct {
	SessionID  string
	UserID     string
	VerifiedAt time.Time
	ExpiresAt  time.Time
}

// APIKey is a service-track credential (plaintext shown once at creation).
type APIKey struct {
	ID              string
	Name            string
	Scope           string // read | write | admin
	KeyHash         string // SHA256 hex of full key string (prefix + secret)
	RevokedAt       *time.Time
	ExpiresAt       *time.Time
	CreatedByUserID *string
	LastUsedAt      *time.Time
	LastUsedIP      string
	CreatedAt       time.Time
}

// APIKeyAuditRow is one programmatic API call audit entry.
type APIKeyAuditRow struct {
	ID       int64
	APIKeyID string
	Method   string
	Path     string
	ClientIP string
	CalledAt time.Time
}
