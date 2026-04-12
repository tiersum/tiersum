package service

import (
	"context"
)

// IAdminConfigViewService exposes read-only, redacted configuration for admin troubleshooting.
type IAdminConfigViewService interface {
	// RedactedSnapshot returns the effective merged settings (viper) with sensitive values replaced.
	// actor must be a browser principal with admin role.
	RedactedSnapshot(ctx context.Context, actor *BrowserPrincipal) (map[string]interface{}, error)
}
