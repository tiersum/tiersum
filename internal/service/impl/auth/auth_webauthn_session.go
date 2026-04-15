package auth

import (
	"time"

	gwa "github.com/go-webauthn/webauthn/webauthn"
)

type webAuthnSessionKind string

const (
	webAuthnSessionRegistration webAuthnSessionKind = "registration"
	webAuthnSessionLogin        webAuthnSessionKind = "login"
)

type webAuthnSessionEntry struct {
	kind webAuthnSessionKind
	data gwa.SessionData
	exp  time.Time

	// deviceName is only used for registration ceremonies (friendly label in DB).
	deviceName string
}
