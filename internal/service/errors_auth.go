package service

import "errors"

var (
	ErrAuthSystemNotInitialized = errors.New("system not initialized")
	ErrAuthAlreadyInitialized   = errors.New("system already initialized")
	ErrAuthInvalidAccessToken   = errors.New("invalid access token")
	ErrAuthInvalidSession       = errors.New("invalid or expired session")
	ErrAuthDeviceLimit          = errors.New("device limit reached")
	ErrAuthForbidden            = errors.New("forbidden")
	ErrAuthInvalidAPIKey        = errors.New("invalid api key")
	ErrAuthAPIKeyRevoked        = errors.New("api key revoked")
	ErrAuthInsufficientScope    = errors.New("insufficient scope")
)
