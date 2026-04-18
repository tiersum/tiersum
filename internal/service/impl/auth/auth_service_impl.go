package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewAuthService constructs a service.IAuthService implementation (browser + admin + bootstrap).
func NewAuthService(
	programAuth service.IProgramAuth,
	state storage.ISystemAuthStateRepository,
	users storage.IAuthUserRepository,
	sessions storage.IBrowserSessionRepository,
	deviceTokens storage.IDeviceTokenRepository,
	passkeys storage.IPasskeyCredentialRepository,
	passkeyVerifs storage.IPasskeySessionVerificationRepository,
	keys storage.IAPIKeyRepository,
	audit storage.IAPIKeyAuditRepository,
	logger *zap.Logger,
) service.IAuthService {
	sessionTTL := viper.GetDuration("auth.browser.session_ttl")
	if sessionTTL <= 0 {
		sessionTTL = 168 * time.Hour
	}
	slideUser := viper.GetDuration("auth.browser.slide_user_token_ttl")
	if slideUser <= 0 {
		slideUser = 168 * time.Hour
	}
	maxDev := viper.GetInt("auth.browser.default_max_devices")
	if maxDev <= 0 {
		maxDev = 3
	}
	deviceTTL := viper.GetDuration("auth.browser.device_token_ttl")
	if deviceTTL <= 0 {
		deviceTTL = 30 * 24 * time.Hour
	}
	passkeySessTTL := viper.GetDuration("auth.passkey.webauthn_session_ttl")
	if passkeySessTTL <= 0 {
		passkeySessTTL = 5 * time.Minute
	}
	passkeyVerifyTTL := viper.GetDuration("auth.passkey.session_verification_ttl")
	if passkeyVerifyTTL <= 0 {
		passkeyVerifyTTL = 12 * time.Hour
	}
	return &authService{
		programAuth:        programAuth,
		state:              state,
		users:              users,
		sessions:           sessions,
		deviceTokens:       deviceTokens,
		passkeys:           passkeys,
		passkeyVerifs:      passkeyVerifs,
		keys:               keys,
		audit:              audit,
		log:                logger,
		sessionTTL:         sessionTTL,
		slideUserTokenTTL:  slideUser,
		defaultMaxDevices:  maxDev,
		deviceTokenTTL:     deviceTTL,
		passkeyWebAuthnTTL: passkeySessTTL,
		passkeyVerifyTTL:   passkeyVerifyTTL,
		webAuthnSessions:   make(map[string]webAuthnSessionEntry),
	}
}

type authService struct {
	programAuth service.IProgramAuth

	state         storage.ISystemAuthStateRepository
	users         storage.IAuthUserRepository
	sessions      storage.IBrowserSessionRepository
	deviceTokens  storage.IDeviceTokenRepository
	passkeys      storage.IPasskeyCredentialRepository
	passkeyVerifs storage.IPasskeySessionVerificationRepository
	keys          storage.IAPIKeyRepository
	audit         storage.IAPIKeyAuditRepository
	log           *zap.Logger

	sessionTTL        time.Duration
	slideUserTokenTTL time.Duration
	defaultMaxDevices int
	deviceTokenTTL    time.Duration

	passkeyWebAuthnTTL time.Duration
	passkeyVerifyTTL   time.Duration

	webAuthnMu       sync.Mutex
	webAuthnSessions map[string]webAuthnSessionEntry
}

func (s *authService) IsSystemInitialized(ctx context.Context) (bool, error) {
	return s.programAuth.IsSystemInitialized(ctx)
}

func (s *authService) ValidateAPIKey(ctx context.Context, bearerToken string) (*service.APIKeyPrincipal, error) {
	return s.programAuth.ValidateAPIKey(ctx, bearerToken)
}

func (s *authService) APIKeyMeetsScope(principal *service.APIKeyPrincipal, requiredScope string) bool {
	return s.programAuth.APIKeyMeetsScope(principal, requiredScope)
}

func (s *authService) RecordAPIKeyUse(ctx context.Context, keyID, method, path, clientIP string) error {
	return s.programAuth.RecordAPIKeyUse(ctx, keyID, method, path, clientIP)
}

func (s *authService) Bootstrap(ctx context.Context, adminUsername string) (*service.BootstrapResult, error) {
	init, err := s.IsSystemInitialized(ctx)
	if err != nil {
		return nil, err
	}
	if init {
		return nil, service.ErrAuthAlreadyInitialized
	}
	adminUsername = strings.TrimSpace(adminUsername)
	if adminUsername == "" {
		return nil, errors.New("admin username required")
	}
	if _, err := s.users.GetByUsername(ctx, adminUsername); err == nil {
		return nil, errors.New("admin username already exists")
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	adminPlain, adminHash, err := newBrowserAccessToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	validUntil := now.Add(s.slideUserTokenTTL)
	u := &storage.AuthUser{
		Username:        adminUsername,
		Role:            types.AuthRoleAdmin,
		AccessTokenHash: adminHash,
		TokenExpiryMode: types.AuthTokenExpirySlide,
		MaxDevices:      s.defaultMaxDevices,
		TokenValidUntil: &validUntil,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	apiPlain, apiHash, err := newAPIKeyPlaintext(types.AuthScopeRead)
	if err != nil {
		return nil, err
	}
	uid := u.ID
	k := &storage.APIKey{
		Name:            "bootstrap-read",
		Scope:           types.AuthScopeRead,
		KeyHash:         apiHash,
		CreatedByUserID: &uid,
	}
	if err := s.keys.Create(ctx, k); err != nil {
		return nil, err
	}
	if err := s.state.MarkInitialized(ctx); err != nil {
		return nil, err
	}
	st, err := s.state.Get(ctx)
	if err != nil {
		return nil, err
	}
	initAt := time.Now().UTC()
	if st.InitializedAt != nil {
		initAt = *st.InitializedAt
	}
	return &service.BootstrapResult{
		AdminUsername:         adminUsername,
		AdminAccessTokenPlain: adminPlain,
		InitialAPIKeyPlain:    apiPlain,
		InitialAPIKeyScope:    types.AuthScopeRead,
		InitializedAt:         initAt,
	}, nil
}

func (s *authService) persistNewBrowserSession(ctx context.Context, userID, fpHash, ipP, uaN, tz string, now time.Time) (sessionCookiePlain string, err error) {
	sessPlain, sessHash, err := newSessionCookieValue()
	if err != nil {
		return "", err
	}
	exp := now.Add(s.sessionTTL)
	sess := &storage.BrowserSession{
		UserID:           userID,
		SessionTokenHash: sessHash,
		FingerprintHash:  fpHash,
		IPPrefix:         ipP,
		UserAgentNorm:    uaN,
		Timezone:         tz,
		ExpiresAt:        exp,
		LastSeenAt:       now,
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return "", err
	}
	return sessPlain, nil
}

func (s *authService) LoginWithAccessToken(ctx context.Context, accessTokenPlain string, fp service.FingerprintInput, remoteIP, userAgent string) (string, error) {
	accessTokenPlain = strings.TrimSpace(accessTokenPlain)
	if accessTokenPlain == "" {
		return "", service.ErrAuthInvalidAccessToken
	}
	h := sha256Hex(accessTokenPlain)
	user, err := s.users.GetByAccessTokenHash(ctx, h)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", service.ErrAuthInvalidAccessToken
		}
		return "", err
	}
	now := time.Now().UTC()
	if user.TokenExpiryMode == types.AuthTokenExpirySlide && user.TokenValidUntil != nil && user.TokenValidUntil.Before(now) {
		return "", service.ErrAuthInvalidAccessToken
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
	newValid := now.Add(s.slideUserTokenTTL)
	if user.TokenExpiryMode == types.AuthTokenExpirySlide {
		if err := s.users.UpdateTokenValidUntil(ctx, user.ID, &newValid); err != nil {
			return "", err
		}
	}
	return sessPlain, nil
}

func (s *authService) ValidateBrowserSession(ctx context.Context, sessionCookiePlain, remoteIP, userAgent string) (*service.BrowserPrincipal, error) {
	sessionCookiePlain = strings.TrimSpace(sessionCookiePlain)
	if sessionCookiePlain == "" {
		return nil, service.ErrAuthInvalidSession
	}
	h := sha256Hex(sessionCookiePlain)
	sess, err := s.sessions.GetBySessionTokenHash(ctx, h)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAuthInvalidSession
		}
		return nil, err
	}
	now := time.Now().UTC()
	if !sess.ExpiresAt.After(now) {
		return nil, service.ErrAuthInvalidSession
	}
	if !sessionRequestLooksConsistent(sess.IPPrefix, sess.UserAgentNorm, remoteIP, userAgent) {
		return nil, service.ErrAuthInvalidSession
	}

	user, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if user.TokenExpiryMode == types.AuthTokenExpirySlide && user.TokenValidUntil != nil && user.TokenValidUntil.Before(now) {
		return nil, service.ErrAuthInvalidSession
	}

	if err := s.sessions.UpdateLastSeen(ctx, sess.ID, now); err != nil {
		return nil, err
	}
	newExp := now.Add(s.sessionTTL)
	if err := s.sessions.UpdateExpiresAt(ctx, sess.ID, newExp); err != nil {
		return nil, err
	}
	if user.TokenExpiryMode == types.AuthTokenExpirySlide {
		nv := now.Add(s.slideUserTokenTTL)
		if err := s.users.UpdateTokenValidUntil(ctx, user.ID, &nv); err != nil {
			return nil, err
		}
	}

	return &service.BrowserPrincipal{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		SessionID: sess.ID,
	}, nil
}

func (s *authService) LogoutSession(ctx context.Context, sessionCookiePlain string) error {
	sessionCookiePlain = strings.TrimSpace(sessionCookiePlain)
	if sessionCookiePlain == "" {
		return nil
	}
	h := sha256Hex(sessionCookiePlain)
	sess, err := s.sessions.GetBySessionTokenHash(ctx, h)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if s.passkeyVerifs != nil {
		_ = s.passkeyVerifs.DeleteBySessionID(ctx, sess.ID)
	}
	return s.sessions.Delete(ctx, sess.ID)
}

func (s *authService) SlideTouchFromBrowserRequest(ctx context.Context, userID string) error {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.TokenExpiryMode != types.AuthTokenExpirySlide {
		return nil
	}
	nv := time.Now().UTC().Add(s.slideUserTokenTTL)
	return s.users.UpdateTokenValidUntil(ctx, userID, &nv)
}

func (s *authService) requireAdmin(actor *service.BrowserPrincipal) error {
	if actor == nil || actor.Role != types.AuthRoleAdmin {
		return service.ErrAuthForbidden
	}
	return nil
}

func (s *authService) CreateUser(ctx context.Context, actor *service.BrowserPrincipal, username, role string) (*service.CreatedSecretOnce, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	role = strings.TrimSpace(role)
	if username == "" || !types.IsValidHumanBrowserRole(role) {
		return nil, errors.New("invalid username or role")
	}
	if _, err := s.users.GetByUsername(ctx, username); err == nil {
		return nil, errors.New("username already exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	plain, hash, err := newBrowserAccessToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	vu := now.Add(s.slideUserTokenTTL)
	u := &storage.AuthUser{
		Username:        username,
		Role:            role,
		AccessTokenHash: hash,
		TokenExpiryMode: types.AuthTokenExpirySlide,
		MaxDevices:      s.defaultMaxDevices,
		TokenValidUntil: &vu,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return &service.CreatedSecretOnce{Plaintext: plain}, nil
}

func (s *authService) ResetUserAccessToken(ctx context.Context, actor *service.BrowserPrincipal, targetUserID string) (*service.CreatedSecretOnce, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	plain, hash, err := newBrowserAccessToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	vu := now.Add(s.slideUserTokenTTL)
	if err := s.users.UpdateAccessToken(ctx, targetUserID, hash, &vu); err != nil {
		return nil, err
	}
	if err := s.sessions.DeleteAllForUser(ctx, targetUserID); err != nil {
		return nil, err
	}
	return &service.CreatedSecretOnce{Plaintext: plain}, nil
}

func (s *authService) ListUsers(ctx context.Context, actor *service.BrowserPrincipal) ([]service.UserSummary, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	rows, err := s.users.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.UserSummary, 0, len(rows))
	for _, u := range rows {
		out = append(out, service.UserSummary{
			ID:              u.ID,
			Username:        u.Username,
			Role:            u.Role,
			TokenExpiryMode: u.TokenExpiryMode,
			MaxDevices:      u.MaxDevices,
			TokenValidUntil: u.TokenValidUntil,
			CreatedAt:       u.CreatedAt,
		})
	}
	return out, nil
}

func summarizeAPIKey(k *storage.APIKey) service.APIKeySummary {
	return service.APIKeySummary{
		ID:              k.ID,
		Name:            k.Name,
		Scope:           k.Scope,
		RevokedAt:       k.RevokedAt,
		ExpiresAt:       k.ExpiresAt,
		CreatedByUserID: k.CreatedByUserID,
		LastUsedAt:      k.LastUsedAt,
		LastUsedIP:      k.LastUsedIP,
		CreatedAt:       k.CreatedAt,
	}
}

func (s *authService) CreateAPIKey(ctx context.Context, actor *service.BrowserPrincipal, name, scope string, expiresAt *time.Time) (*service.CreatedSecretOnce, *service.APIKeySummary, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil, errors.New("name required")
	}
	switch scope {
	case types.AuthScopeRead, types.AuthScopeWrite, types.AuthScopeAdmin:
	default:
		return nil, nil, errors.New("invalid scope")
	}
	plain, hash, err := newAPIKeyPlaintext(scope)
	if err != nil {
		return nil, nil, err
	}
	uid := actor.UserID
	k := &storage.APIKey{
		Name:            name,
		Scope:           scope,
		KeyHash:         hash,
		ExpiresAt:       expiresAt,
		CreatedByUserID: &uid,
	}
	if err := s.keys.Create(ctx, k); err != nil {
		return nil, nil, err
	}
	sum := summarizeAPIKey(k)
	return &service.CreatedSecretOnce{Plaintext: plain}, &sum, nil
}

func (s *authService) RevokeAPIKey(ctx context.Context, actor *service.BrowserPrincipal, keyID string) error {
	if err := s.requireAdmin(actor); err != nil {
		return err
	}
	return s.keys.Revoke(ctx, keyID)
}

func (s *authService) ListAPIKeys(ctx context.Context, actor *service.BrowserPrincipal) ([]service.APIKeySummary, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	rows, err := s.keys.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.APIKeySummary, 0, len(rows))
	for i := range rows {
		out = append(out, summarizeAPIKey(&rows[i]))
	}
	return out, nil
}

func (s *authService) ListOwnDevices(ctx context.Context, actor *service.BrowserPrincipal) ([]service.BrowserDeviceSummary, error) {
	if actor == nil {
		return nil, service.ErrAuthForbidden
	}
	return s.listDevicesForUser(ctx, actor.UserID)
}

func (s *authService) ListUserDevicesAdmin(ctx context.Context, actor *service.BrowserPrincipal, targetUserID string) ([]service.BrowserDeviceSummary, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	return s.listDevicesForUser(ctx, targetUserID)
}

func (s *authService) ListAllDevicesAdmin(ctx context.Context, actor *service.BrowserPrincipal) ([]service.AdminBrowserDeviceSummary, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	rows, err := s.sessions.ListAllWithUsername(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.AdminBrowserDeviceSummary, 0, len(rows))
	for _, row := range rows {
		se := row.BrowserSession
		out = append(out, service.AdminBrowserDeviceSummary{
			UserID:      se.UserID,
			Username:    row.Username,
			SessionID:   se.ID,
			DeviceAlias: se.DeviceAlias,
			IPPrefix:    se.IPPrefix,
			LastSeenAt:  se.LastSeenAt,
			CreatedAt:   se.CreatedAt,
			ExpiresAt:   se.ExpiresAt,
		})
	}
	return out, nil
}

func (s *authService) listDevicesForUser(ctx context.Context, userID string) ([]service.BrowserDeviceSummary, error) {
	sessions, err := s.sessions.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]service.BrowserDeviceSummary, 0, len(sessions))
	for _, se := range sessions {
		out = append(out, service.BrowserDeviceSummary{
			SessionID:   se.ID,
			DeviceAlias: se.DeviceAlias,
			IPPrefix:    se.IPPrefix,
			LastSeenAt:  se.LastSeenAt,
			CreatedAt:   se.CreatedAt,
			ExpiresAt:   se.ExpiresAt,
		})
	}
	return out, nil
}

func (s *authService) UpdateDeviceAlias(ctx context.Context, actor *service.BrowserPrincipal, sessionID, alias string) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	sess, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.UserID != actor.UserID && actor.Role != types.AuthRoleAdmin {
		return service.ErrAuthForbidden
	}
	return s.sessions.UpdateDeviceAlias(ctx, sessionID, alias)
}

func (s *authService) RevokeDeviceSession(ctx context.Context, actor *service.BrowserPrincipal, sessionID string) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	sess, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.UserID != actor.UserID && actor.Role != types.AuthRoleAdmin {
		return service.ErrAuthForbidden
	}
	return s.sessions.Delete(ctx, sessionID)
}

func (s *authService) RevokeAllOwnSessions(ctx context.Context, actor *service.BrowserPrincipal) error {
	if actor == nil {
		return service.ErrAuthForbidden
	}
	if s.deviceTokens != nil {
		_ = s.deviceTokens.RevokeAllForUser(ctx, actor.UserID, time.Now().UTC())
	}
	if s.passkeyVerifs != nil {
		_ = s.passkeyVerifs.DeleteBySessionID(ctx, actor.SessionID)
	}
	return s.sessions.DeleteAllForUser(ctx, actor.UserID)
}

func (s *authService) APIKeyUsageCountsSince(ctx context.Context, actor *service.BrowserPrincipal, since time.Time) (map[string]int64, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	return s.audit.CountsPerKeySince(ctx, since)
}

var (
	_ service.IProgramAuth              = (*authService)(nil)
	_ service.IAuthBootstrap            = (*authService)(nil)
	_ service.IBrowserSessionValidator  = (*authService)(nil)
	_ service.IBFFSessionMiddlewareAuth = (*authService)(nil)
	_ service.IBrowserCredentialAuth    = (*authService)(nil)
	_ service.IAdminAuthDirectory       = (*authService)(nil)
	_ service.IPasskeyPolicyReader      = (*authService)(nil)
	_ service.IPasskeyAuth              = (*authService)(nil)
	_ service.IAuthService              = (*authService)(nil)
)
