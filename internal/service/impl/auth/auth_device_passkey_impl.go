package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/go-webauthn/webauthn/protocol"
	gwa "github.com/go-webauthn/webauthn/webauthn"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

func (s *authService) putWebAuthnSessionLocked(id string, kind webAuthnSessionKind, data gwa.SessionData, deviceName string) {
	if s.webAuthnSessions == nil {
		s.webAuthnSessions = make(map[string]webAuthnSessionEntry)
	}
	s.webAuthnSessions[id] = webAuthnSessionEntry{
		kind:       kind,
		data:       data,
		exp:        time.Now().UTC().Add(s.passkeyWebAuthnTTL),
		deviceName: strings.TrimSpace(deviceName),
	}
}

func (s *authService) storeWebAuthnSession(kind webAuthnSessionKind, data gwa.SessionData, deviceName string) string {
	id := uuid.New().String()
	s.webAuthnMu.Lock()
	defer s.webAuthnMu.Unlock()
	s.putWebAuthnSessionLocked(id, kind, data, deviceName)
	return id
}

func (s *authService) takeWebAuthnSession(id string, kind webAuthnSessionKind) (gwa.SessionData, string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return gwa.SessionData{}, "", errors.New("missing webauthn session id")
	}
	now := time.Now().UTC()
	s.webAuthnMu.Lock()
	defer s.webAuthnMu.Unlock()
	if s.webAuthnSessions == nil {
		return gwa.SessionData{}, "", errors.New("webauthn session store unavailable")
	}
	ent, ok := s.webAuthnSessions[id]
	if !ok || ent.kind != kind {
		return gwa.SessionData{}, "", errors.New("unknown or expired webauthn session")
	}
	if ent.exp.Before(now) {
		delete(s.webAuthnSessions, id)
		return gwa.SessionData{}, "", errors.New("unknown or expired webauthn session")
	}
	delete(s.webAuthnSessions, id)
	return ent.data, ent.deviceName, nil
}

func (s *authService) passkeyAdminRequired() bool {
	return viper.GetBool("auth.passkey.admin_required")
}

func splitOriginsCSV(csv string) []string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *authService) newWebAuthn(rpID, origin string) (*gwa.WebAuthn, error) {
	rpID = strings.TrimSpace(rpID)
	if rpID == "" {
		return nil, errors.New("rp_id is required")
	}
	rpName := strings.TrimSpace(viper.GetString("auth.passkey.rp_display_name"))
	if rpName == "" {
		rpName = "TierSum"
	}
	origins := splitOriginsCSV(viper.GetString("auth.passkey.rp_origins"))
	if len(origins) == 0 && strings.TrimSpace(origin) != "" {
		origins = []string{strings.TrimSpace(origin)}
	}
	if len(origins) == 0 {
		return nil, errors.New("auth.passkey.rp_origins must be configured (comma-separated), or pass a valid Origin")
	}
	cfg := &gwa.Config{
		RPID:                 rpID,
		RPDisplayName:        rpName,
		RPOrigins:            origins,
		EncodeUserIDAsString: true,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: gwa.TimeoutsConfig{
			Registration: gwa.TimeoutConfig{
				Enforce:    true,
				Timeout:    2 * time.Minute,
				TimeoutUVD: 2 * time.Minute,
			},
			Login: gwa.TimeoutConfig{
				Enforce:    true,
				Timeout:    2 * time.Minute,
				TimeoutUVD: 2 * time.Minute,
			},
		},
	}
	return gwa.New(cfg)
}

type webAuthnUser struct {
	id          string
	name        string
	displayName string
	creds       []gwa.Credential
}

func (u webAuthnUser) WebAuthnID() []byte {
	return []byte(u.id)
}

func (u webAuthnUser) WebAuthnName() string {
	return u.name
}

func (u webAuthnUser) WebAuthnDisplayName() string {
	if strings.TrimSpace(u.displayName) != "" {
		return u.displayName
	}
	return u.name
}

func (u webAuthnUser) WebAuthnCredentials() []gwa.Credential {
	return u.creds
}

func decodeB64URL(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty base64")
	}
	return base64.RawURLEncoding.DecodeString(s)
}

func (s *authService) webAuthnUserFromDB(ctx context.Context, userID string) (webAuthnUser, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return webAuthnUser{}, err
	}
	rows, err := s.passkeys.ListByUser(ctx, userID)
	if err != nil {
		return webAuthnUser{}, err
	}
	creds := make([]gwa.Credential, 0, len(rows))
	for _, row := range rows {
		credID, err := decodeB64URL(row.CredentialIDB64)
		if err != nil {
			continue
		}
		pubKey, err := decodeB64URL(row.PublicKeyB64)
		if err != nil {
			continue
		}
		creds = append(creds, gwa.Credential{
			ID:        credID,
			PublicKey: pubKey,
			Authenticator: gwa.Authenticator{
				SignCount: uint32(row.SignCount),
			},
		})
	}
	return webAuthnUser{
		id:          u.ID,
		name:        u.Username,
		displayName: u.Username,
		creds:       creds,
	}, nil
}

func (s *authService) upsertPasskeyVerification(ctx context.Context, sessionID, userID string) error {
	if s.passkeyVerifs == nil {
		return nil
	}
	now := time.Now().UTC()
	v := &storage.PasskeySessionVerification{
		SessionID:  sessionID,
		UserID:     userID,
		VerifiedAt: now,
		ExpiresAt:  now.Add(s.passkeyVerifyTTL),
	}
	return s.passkeyVerifs.Put(ctx, v)
}

func (s *authService) DeviceLogin(ctx context.Context, deviceTokenPlain string, fp service.FingerprintInput, remoteIP, userAgent string) (string, error) {
	if s.deviceTokens == nil {
		return "", service.ErrAuthInvalidDeviceToken
	}
	deviceTokenPlain = strings.TrimSpace(deviceTokenPlain)
	if deviceTokenPlain == "" {
		return "", service.ErrAuthInvalidDeviceToken
	}
	th := sha256Hex(deviceTokenPlain)
	dt, err := s.deviceTokens.GetByTokenHash(ctx, th)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", service.ErrAuthInvalidDeviceToken
		}
		return "", err
	}
	now := time.Now().UTC()
	if dt.RevokedAt != nil || !dt.ExpiresAt.After(now) {
		return "", service.ErrAuthInvalidDeviceToken
	}
	if !sessionRequestLooksConsistent(dt.IPPrefix, dt.UserAgentNorm, remoteIP, userAgent) {
		return "", service.ErrAuthInvalidDeviceToken
	}

	user, err := s.users.GetByID(ctx, dt.UserID)
	if err != nil {
		return "", err
	}
	if user.TokenExpiryMode == types.AuthTokenExpirySlide && user.TokenValidUntil != nil && user.TokenValidUntil.Before(now) {
		return "", service.ErrAuthInvalidDeviceToken
	}

	ipP := ipPrefixForBind(remoteIP)
	uaN := normalizeUserAgent(userAgent)
	fpHash := fingerprintStrictHash(userAgent, ipP, fp.Timezone, fp.ClientSignal)

	has, err := s.sessions.HasActiveSessionWithFingerprint(ctx, user.ID, fpHash, now)
	if err != nil {
		return "", err
	}
	if !has {
		n, err := s.sessions.CountActiveDistinctFingerprints(ctx, user.ID, now)
		if err != nil {
			return "", err
		}
		if n >= user.MaxDevices {
			return "", service.ErrAuthDeviceLimit
		}
	}
	if err := s.sessions.DeleteByUserAndFingerprint(ctx, user.ID, fpHash); err != nil {
		return "", err
	}

	sessPlain, err := s.persistNewBrowserSession(ctx, user.ID, fpHash, ipP, uaN, fp.Timezone, now)
	if err != nil {
		return "", err
	}
	if err := s.deviceTokens.TouchUse(ctx, dt.ID, now); err != nil {
		// Best-effort; login still succeeds.
		_ = err
	}

	newValid := now.Add(s.slideUserTokenTTL)
	if user.TokenExpiryMode == types.AuthTokenExpirySlide {
		if err := s.users.UpdateTokenValidUntil(ctx, user.ID, &newValid); err != nil {
			return "", err
		}
	}
	return sessPlain, nil
}

func (s *authService) CreateDeviceTokenForSession(ctx context.Context, actor *service.BrowserPrincipal, deviceName, remoteIP, userAgent string) (string, error) {
	if actor == nil {
		return "", service.ErrAuthForbidden
	}
	if s.deviceTokens == nil {
		return "", errors.New("device tokens unavailable")
	}
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		deviceName = "browser"
	}
	plain, hash, err := newDeviceTokenPlaintext()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	row := &storage.DeviceToken{
		UserID:        actor.UserID,
		TokenHash:     hash,
		DeviceName:    deviceName,
		IPPrefix:      ipPrefixForBind(remoteIP),
		UserAgentNorm: normalizeUserAgent(userAgent),
		ExpiresAt:     now.Add(s.deviceTokenTTL),
		CreatedAt:     now,
	}
	if err := s.deviceTokens.Create(ctx, row); err != nil {
		return "", err
	}
	return plain, nil
}

func (s *authService) ListOwnDeviceTokens(ctx context.Context, actor *service.BrowserPrincipal) ([]service.DeviceTokenSummary, error) {
	if actor == nil {
		return nil, service.ErrAuthForbidden
	}
	if s.deviceTokens == nil {
		return []service.DeviceTokenSummary{}, nil
	}
	rows, err := s.deviceTokens.ListByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]service.DeviceTokenSummary, 0, len(rows))
	for _, t := range rows {
		if t.RevokedAt != nil {
			continue
		}
		out = append(out, service.DeviceTokenSummary{
			ID:         t.ID,
			DeviceName: t.DeviceName,
			IPPrefix:   t.IPPrefix,
			LastUsedAt: t.LastUsedAt,
			ExpiresAt:  t.ExpiresAt,
			CreatedAt:  t.CreatedAt,
		})
	}
	return out, nil
}

func (s *authService) RevokeDeviceToken(ctx context.Context, actor *service.BrowserPrincipal, tokenID string) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	if s.deviceTokens == nil {
		return errors.New("device tokens unavailable")
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return errors.New("token id required")
	}
	rows, err := s.deviceTokens.ListByUser(ctx, actor.UserID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if row.ID != tokenID {
			continue
		}
		if row.RevokedAt != nil {
			return nil
		}
		return s.deviceTokens.Revoke(ctx, row.ID, now)
	}
	return errors.New("device token not found")
}

func (s *authService) RevokeAllOwnDeviceTokens(ctx context.Context, actor *service.BrowserPrincipal) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	if s.deviceTokens == nil {
		return nil
	}
	return s.deviceTokens.RevokeAllForUser(ctx, actor.UserID, time.Now().UTC())
}

func (s *authService) PasskeyStatus(ctx context.Context, actor *service.BrowserPrincipal) (*service.PasskeyStatus, error) {
	if actor == nil {
		return nil, service.ErrAuthForbidden
	}
	if s.passkeys == nil || s.passkeyVerifs == nil {
		return &service.PasskeyStatus{RequiredForAdmin: s.passkeyAdminRequired()}, nil
	}
	rows, err := s.passkeys.ListByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	has := len(rows) > 0
	var verifiedAt *time.Time
	if has {
		if v, err := s.passkeyVerifs.GetBySessionID(ctx, actor.SessionID); err == nil && v != nil {
			now := time.Now().UTC()
			if v.ExpiresAt.After(now) {
				verifiedAt = &v.VerifiedAt
			}
		}
	}
	return &service.PasskeyStatus{
		HasAny:           has,
		VerifiedAt:       verifiedAt,
		RequiredForAdmin: s.passkeyAdminRequired(),
	}, nil
}

func (s *authService) ListPasskeys(ctx context.Context, actor *service.BrowserPrincipal) ([]service.PasskeySummary, error) {
	if actor == nil {
		return nil, service.ErrAuthForbidden
	}
	if s.passkeys == nil {
		return []service.PasskeySummary{}, nil
	}
	rows, err := s.passkeys.ListByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]service.PasskeySummary, 0, len(rows))
	for _, c := range rows {
		out = append(out, service.PasskeySummary{
			ID:         c.ID,
			DeviceName: c.DeviceName,
			CreatedAt:  c.CreatedAt,
			LastUsedAt: c.LastUsedAt,
		})
	}
	return out, nil
}

func (s *authService) RevokePasskey(ctx context.Context, actor *service.BrowserPrincipal, passkeyID string) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	if s.passkeys == nil {
		return errors.New("passkeys unavailable")
	}
	passkeyID = strings.TrimSpace(passkeyID)
	if passkeyID == "" {
		return errors.New("passkey id required")
	}
	c, err := s.passkeys.GetByID(ctx, passkeyID)
	if err != nil {
		return err
	}
	if c.UserID != actor.UserID {
		return service.ErrAuthForbidden
	}
	return s.passkeys.Delete(ctx, passkeyID)
}

type passkeyFinishEnvelope struct {
	SessionID  string          `json:"session_id"`
	DeviceName string          `json:"device_name,omitempty"`
	Credential json.RawMessage `json:"credential"`
}

func httpRequestFromCredentialJSON(credentialJSON []byte) (*http.Request, error) {
	credentialJSON = bytes.TrimSpace(credentialJSON)
	if len(credentialJSON) == 0 {
		return nil, errors.New("credential json required")
	}
	req, err := http.NewRequest(http.MethodPost, "/", io.NopCloser(bytes.NewReader(credentialJSON)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (s *authService) BeginPasskeyRegistration(ctx context.Context, actor *service.BrowserPrincipal, rpID, origin, deviceName string) (any, error) {
	if actor == nil {
		return nil, service.ErrAuthForbidden
	}
	if s.passkeys == nil {
		return nil, errors.New("passkeys unavailable")
	}
	wa, err := s.newWebAuthn(rpID, origin)
	if err != nil {
		return nil, err
	}
	u, err := s.webAuthnUserFromDB(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	excl := make([]protocol.CredentialDescriptor, 0, len(u.creds))
	for _, c := range u.creds {
		excl = append(excl, c.Descriptor())
	}
	creation, session, err := wa.BeginRegistration(
		u,
		gwa.WithExclusions(excl),
	)
	if err != nil {
		return nil, err
	}
	sid := s.storeWebAuthnSession(webAuthnSessionRegistration, *session, deviceName)
	return map[string]any{
		"session_id":  sid,
		"device_name": strings.TrimSpace(deviceName),
		"publicKey":   creation,
	}, nil
}

func (s *authService) FinishPasskeyRegistration(ctx context.Context, actor *service.BrowserPrincipal, credential any) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	if s.passkeys == nil {
		return errors.New("passkeys unavailable")
	}
	raw, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	var env passkeyFinishEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	sess, regDeviceName, err := s.takeWebAuthnSession(env.SessionID, webAuthnSessionRegistration)
	if err != nil {
		return err
	}
	wa, err := s.newWebAuthn(sess.RelyingPartyID, "")
	if err != nil {
		return err
	}
	u, err := s.webAuthnUserFromDB(ctx, actor.UserID)
	if err != nil {
		return err
	}
	req, err := httpRequestFromCredentialJSON(env.Credential)
	if err != nil {
		return err
	}
	cred, err := wa.FinishRegistration(u, sess, req)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	dn := strings.TrimSpace(env.DeviceName)
	if dn == "" {
		dn = strings.TrimSpace(regDeviceName)
	}
	if dn == "" {
		dn = "passkey"
	}
	row := &storage.PasskeyCredential{
		UserID:          actor.UserID,
		CredentialIDB64: base64.RawURLEncoding.EncodeToString(cred.ID),
		PublicKeyB64:    base64.RawURLEncoding.EncodeToString(cred.PublicKey),
		SignCount:       int64(cred.Authenticator.SignCount),
		DeviceName:      dn,
		CreatedAt:       now,
	}
	if err := s.passkeys.Create(ctx, row); err != nil {
		return err
	}
	return s.upsertPasskeyVerification(ctx, actor.SessionID, actor.UserID)
}

func (s *authService) BeginPasskeyVerification(ctx context.Context, actor *service.BrowserPrincipal, rpID, origin string) (any, error) {
	if actor == nil {
		return nil, service.ErrAuthForbidden
	}
	if s.passkeys == nil {
		return nil, errors.New("passkeys unavailable")
	}
	wa, err := s.newWebAuthn(rpID, origin)
	if err != nil {
		return nil, err
	}
	u, err := s.webAuthnUserFromDB(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if len(u.creds) == 0 {
		return nil, errors.New("no passkeys registered")
	}
	assertion, session, err := wa.BeginLogin(u)
	if err != nil {
		return nil, err
	}
	sid := s.storeWebAuthnSession(webAuthnSessionLogin, *session, "")
	return map[string]any{
		"session_id": sid,
		"publicKey":  assertion,
	}, nil
}

func (s *authService) FinishPasskeyVerification(ctx context.Context, actor *service.BrowserPrincipal, assertion any) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	if s.passkeys == nil {
		return errors.New("passkeys unavailable")
	}
	raw, err := json.Marshal(assertion)
	if err != nil {
		return err
	}
	var env passkeyFinishEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	sess, _, err := s.takeWebAuthnSession(env.SessionID, webAuthnSessionLogin)
	if err != nil {
		return err
	}
	wa, err := s.newWebAuthn(sess.RelyingPartyID, "")
	if err != nil {
		return err
	}
	u, err := s.webAuthnUserFromDB(ctx, actor.UserID)
	if err != nil {
		return err
	}
	req, err := httpRequestFromCredentialJSON(env.Credential)
	if err != nil {
		return err
	}
	cred, err := wa.FinishLogin(u, sess, req)
	if err != nil {
		return err
	}
	stored, err := s.passkeys.GetByCredentialID(ctx, base64.RawURLEncoding.EncodeToString(cred.ID))
	if err != nil {
		return err
	}
	if stored.UserID != actor.UserID {
		return service.ErrAuthForbidden
	}
	now := time.Now().UTC()
	if err := s.passkeys.UpdateSignCountAndLastUsed(ctx, stored.ID, int64(cred.Authenticator.SignCount), now); err != nil {
		return err
	}
	return s.upsertPasskeyVerification(ctx, actor.SessionID, actor.UserID)
}
