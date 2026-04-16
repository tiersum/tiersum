package api

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

func init() {
	viper.SetDefault("auth.bootstrap.allow_remote", false)
	viper.SetDefault("auth.browser.cookie_secure_mode", "auto") // auto|always|never
	viper.SetDefault("auth.browser.trust_proxy_headers", false)
	viper.SetDefault("auth.passkey.admin_required", true)
	viper.SetDefault("auth.passkey.rp_display_name", "TierSum")
}

// AuthBFFHandler registers human-track bootstrap, login, and admin JSON endpoints under /bff/v1.
type AuthBFFHandler struct {
	Auth          service.IAuthService
	ConfigView    service.IAdminConfigViewService
	Logger        *zap.Logger
	ServerVersion string

	bootstrapLimiter *ipRateLimiter
	loginLimiter     *ipRateLimiter
	loginBackoff     *loginFailBackoff
}

// NewAuthBFFHandler constructs browser-facing auth handlers.
func NewAuthBFFHandler(auth service.IAuthService, configView service.IAdminConfigViewService, logger *zap.Logger, serverVersion string) *AuthBFFHandler {
	return &AuthBFFHandler{
		Auth:             auth,
		ConfigView:       configView,
		Logger:           logger,
		ServerVersion:    strings.TrimSpace(serverVersion),
		bootstrapLimiter: newIPRateLimiter(ipRateConfig{Capacity: 2, RefillPerSec: 1.0 / 60.0, EntryTTL: 30 * time.Minute}), // 1/min, burst 2
		loginLimiter:     newIPRateLimiter(ipRateConfig{Capacity: 8, RefillPerSec: 8.0 / 60.0, EntryTTL: 30 * time.Minute}), // 8/min, burst 8
		loginBackoff:     newLoginFailBackoff(30 * time.Minute),
	}
}

// RegisterPublicRoutes mounts unauthenticated paths on the given group (must be /bff/v1).
func (h *AuthBFFHandler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/system/status", h.GetSystemStatus)
	rg.POST("/system/bootstrap", h.PostBootstrap)
	rg.POST("/auth/login", h.PostLogin)
	rg.POST("/auth/device_login", h.PostDeviceLogin)
	rg.POST("/auth/logout", h.PostLogout)
}

// RegisterAdminRoutes mounts /bff/v1/admin/* (caller must apply BFFSessionMiddleware + BFFRequireAdmin).
func (h *AuthBFFHandler) RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.GET("/users", h.AdminListUsers)
	rg.POST("/users", h.AdminCreateUser)
	rg.POST("/users/:id/reset_token", h.AdminResetUserToken)
	// Static path before /users/:id/devices so the router never treats "devices" as :id.
	rg.GET("/devices", h.AdminListAllDevices)
	rg.GET("/users/:id/devices", h.AdminListUserDevices)

	rg.GET("/api_keys", h.AdminListAPIKeys)
	rg.GET("/api_keys/usage", h.AdminAPIKeyUsage)
	rg.POST("/api_keys", h.AdminCreateAPIKey)
	rg.POST("/api_keys/:id/revoke", h.AdminRevokeAPIKey)

	rg.GET("/config/snapshot", h.AdminConfigSnapshot)
}

// RegisterMeRoutes mounts /bff/v1/me/* (caller must apply BFFSessionMiddleware).
func (h *AuthBFFHandler) RegisterMeRoutes(rg *gin.RouterGroup) {
	rg.GET("/profile", h.MeProfile)
	rg.GET("/devices", h.MeListDevices)
	rg.PATCH("/devices/:id/alias", h.MePatchDeviceAlias)
	rg.DELETE("/devices/:id", h.MeDeleteDevice)
	rg.POST("/sessions/revoke_all", h.MeRevokeAllSessions)

	rg.GET("/security/passkeys/status", h.MePasskeyStatus)
	rg.GET("/security/passkeys", h.MeListPasskeys)
	rg.DELETE("/security/passkeys/:id", h.MeDeletePasskey)
	rg.POST("/security/passkeys/registration/begin", h.MePasskeyRegistrationBegin)
	rg.POST("/security/passkeys/registration/finish", h.MePasskeyRegistrationFinish)
	rg.POST("/security/passkeys/verification/begin", h.MePasskeyVerificationBegin)
	rg.POST("/security/passkeys/verification/finish", h.MePasskeyVerificationFinish)

	rg.GET("/security/device_tokens", h.MeListDeviceTokens)
	rg.POST("/security/device_tokens", h.MeCreateDeviceToken)
	rg.DELETE("/security/device_tokens/:id", h.MeRevokeDeviceToken)
	rg.POST("/security/device_tokens/revoke_all", h.MeRevokeAllDeviceTokens)
}

func (h *AuthBFFHandler) GetSystemStatus(c *gin.Context) {
	ctx := c.Request.Context()
	init, err := h.Auth.IsSystemInitialized(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth_state_unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"initialized": init,
		"version":     h.ServerVersion,
	})
}

type bootstrapBody struct {
	Username string `json:"username"`
}

func (h *AuthBFFHandler) PostBootstrap(c *gin.Context) {
	ctx := c.Request.Context()
	if !viper.GetBool("auth.bootstrap.allow_remote") {
		ip := net.ParseIP(normalizeClientIP(c.ClientIP()))
		if ip == nil || !(ip.IsLoopback() || isPrivateIP(ip)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "bootstrap_remote_forbidden"})
			return
		}
	}
	if h.bootstrapLimiter != nil && !h.bootstrapLimiter.Allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
		return
	}
	init, err := h.Auth.IsSystemInitialized(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth_state_unavailable"})
		return
	}
	if init {
		c.JSON(http.StatusForbidden, gin.H{"error": "already_initialized"})
		return
	}
	var body bootstrapBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.Auth.Bootstrap(ctx, body.Username)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("bootstrap failed", zap.Error(err))
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"username":              res.AdminUsername,
		"admin_access_token":    res.AdminAccessTokenPlain,
		"initial_api_key":       res.InitialAPIKeyPlain,
		"initial_api_key_scope": res.InitialAPIKeyScope,
		"initialized_at":        res.InitializedAt,
	})
}

type loginBody struct {
	AccessToken string                   `json:"access_token"`
	Fingerprint service.FingerprintInput `json:"fingerprint"`
	RememberMe  bool                     `json:"remember_me"`
	DeviceName  string                   `json:"device_name"`
}

func (h *AuthBFFHandler) PostLogin(c *gin.Context) {
	ctx := c.Request.Context()
	if h.loginLimiter != nil && !h.loginLimiter.Allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
		return
	}
	if h.loginBackoff != nil {
		if ok, until := h.loginBackoff.Allowed(c.ClientIP()); !ok {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "try_later", "retry_after": until.UTC().Format(time.RFC3339)})
			return
		}
	}
	var body loginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sess, err := h.Auth.LoginWithAccessToken(ctx, body.AccessToken, body.Fingerprint, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if h.loginBackoff != nil {
			h.loginBackoff.RecordFailure(c.ClientIP())
		}
		switch err {
		case service.ErrAuthInvalidAccessToken:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_key"})
		case service.ErrAuthDeviceLimit:
			c.JSON(http.StatusForbidden, gin.H{"error": "device_limit"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	if h.loginBackoff != nil {
		h.loginBackoff.Reset(c.ClientIP())
	}
	// HttpOnly session cookie; Secure only when TLS (caller may terminate TLS upstream).
	secure := requestShouldSetSecureCookie(c)
	ttl := viper.GetDuration("auth.browser.session_ttl")
	if ttl <= 0 {
		ttl = 168 * time.Hour
	}
	maxAge := int(ttl.Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, sess, maxAge, "/", "", secure, true)

	if body.RememberMe {
		p, perr := h.Auth.ValidateBrowserSession(ctx, sess, c.ClientIP(), c.Request.UserAgent())
		if perr == nil && p != nil {
			dname := strings.TrimSpace(body.DeviceName)
			if dname == "" {
				dname = "browser"
			}
			if dt, derr := h.Auth.CreateDeviceTokenForSession(ctx, p, dname, c.ClientIP(), c.Request.UserAgent()); derr == nil && strings.TrimSpace(dt) != "" {
				dTTL := viper.GetDuration("auth.browser.device_token_ttl")
				if dTTL <= 0 {
					dTTL = 30 * 24 * time.Hour
				}
				c.SetSameSite(http.SameSiteLaxMode)
				c.SetCookie(deviceCookieName, dt, int(dTTL.Seconds()), "/", "", secure, true)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type deviceLoginBody struct {
	DeviceToken string                   `json:"device_token"`
	Fingerprint service.FingerprintInput `json:"fingerprint"`
}

func (h *AuthBFFHandler) PostDeviceLogin(c *gin.Context) {
	ctx := c.Request.Context()
	if h.loginLimiter != nil && !h.loginLimiter.Allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
		return
	}
	if h.loginBackoff != nil {
		if ok, until := h.loginBackoff.Allowed(c.ClientIP()); !ok {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "try_later", "retry_after": until.UTC().Format(time.RFC3339)})
			return
		}
	}

	token := ""
	var body deviceLoginBody
	if err := c.ShouldBindJSON(&body); err == nil {
		token = strings.TrimSpace(body.DeviceToken)
	}
	if token == "" {
		if ck, err := c.Cookie(deviceCookieName); err == nil {
			token = strings.TrimSpace(ck)
		}
	}
	sess, err := h.Auth.DeviceLogin(ctx, token, body.Fingerprint, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if h.loginBackoff != nil {
			h.loginBackoff.RecordFailure(c.ClientIP())
		}
		switch err {
		case service.ErrAuthInvalidDeviceToken:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_device_token"})
		case service.ErrAuthDeviceLimit:
			c.JSON(http.StatusForbidden, gin.H{"error": "device_limit"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	if h.loginBackoff != nil {
		h.loginBackoff.Reset(c.ClientIP())
	}
	secure := requestShouldSetSecureCookie(c)
	ttl := viper.GetDuration("auth.browser.session_ttl")
	if ttl <= 0 {
		ttl = 168 * time.Hour
	}
	maxAge := int(ttl.Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, sess, maxAge, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) PostLogout(c *gin.Context) {
	ctx := c.Request.Context()
	cookie, err := c.Cookie(sessionCookieName)
	if err == nil && strings.TrimSpace(cookie) != "" {
		_ = h.Auth.LogoutSession(ctx, cookie)
	}
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", secure, true)
	c.SetCookie(deviceCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func requestShouldSetSecureCookie(c *gin.Context) bool {
	mode := strings.ToLower(strings.TrimSpace(viper.GetString("auth.browser.cookie_secure_mode")))
	switch mode {
	case "always", "true", "1", "yes", "on":
		return true
	case "never", "false", "0", "off":
		return false
	default:
		// auto
		if c.Request.TLS != nil {
			return true
		}
		if !viper.GetBool("auth.browser.trust_proxy_headers") {
			return false
		}
		if strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https") {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Ssl")), "on") {
			return true
		}
		return false
	}
}

func bffWebAuthnRP(c *gin.Context) (rpID string, origin string) {
	if v := strings.TrimSpace(viper.GetString("auth.passkey.rp_id")); v != "" {
		rpID = v
	}
	origin = strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return rpID, origin
	}
	u, err := url.Parse(origin)
	if err != nil {
		return rpID, origin
	}
	if rpID == "" {
		rpID = strings.TrimSpace(u.Hostname())
	}
	return rpID, origin
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// IPv4 private ranges.
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 10:
			return true
		case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
			return true
		case v4[0] == 192 && v4[1] == 168:
			return true
		case v4[0] == 127:
			return true
		default:
			return false
		}
	}
	// IPv6: unique local fc00::/7 and loopback.
	if ip.IsLoopback() {
		return true
	}
	return len(ip) >= 2 && (ip[0]&0xfe) == 0xfc
}

func (h *AuthBFFHandler) AdminListUsers(c *gin.Context) {
	rows, err := h.Auth.ListUsers(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": rows})
}

type createUserBody struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (h *AuthBFFHandler) AdminCreateUser(c *gin.Context) {
	var body createUserBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	once, err := h.Auth.CreateUser(c.Request.Context(), BrowserPrincipal(c), body.Username, body.Role)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": once.Plaintext})
}

func (h *AuthBFFHandler) AdminResetUserToken(c *gin.Context) {
	id := c.Param("id")
	once, err := h.Auth.ResetUserAccessToken(c.Request.Context(), BrowserPrincipal(c), id)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": once.Plaintext})
}

func (h *AuthBFFHandler) AdminListUserDevices(c *gin.Context) {
	id := c.Param("id")
	rows, err := h.Auth.ListUserDevicesAdmin(c.Request.Context(), BrowserPrincipal(c), id)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"devices": rows})
}

func (h *AuthBFFHandler) AdminListAllDevices(c *gin.Context) {
	rows, err := h.Auth.ListAllDevicesAdmin(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"devices": rows})
}

func (h *AuthBFFHandler) AdminListAPIKeys(c *gin.Context) {
	rows, err := h.Auth.ListAPIKeys(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_keys": rows})
}

type createAPIKeyBody struct {
	Name      string     `json:"name"`
	Scope     string     `json:"scope"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (h *AuthBFFHandler) AdminCreateAPIKey(c *gin.Context) {
	var body createAPIKeyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	once, meta, err := h.Auth.CreateAPIKey(c.Request.Context(), BrowserPrincipal(c), body.Name, body.Scope, body.ExpiresAt)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_key": once.Plaintext, "meta": meta})
}

func (h *AuthBFFHandler) AdminRevokeAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.Auth.RevokeAPIKey(c.Request.Context(), BrowserPrincipal(c), id); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) AdminConfigSnapshot(c *gin.Context) {
	if h.ConfigView == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config_view_unavailable"})
		return
	}
	snap, err := h.ConfigView.RedactedSnapshot(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"snapshot":     snap,
		"source":       "viper_effective",
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *AuthBFFHandler) AdminAPIKeyUsage(c *gin.Context) {
	days := 7
	if v := strings.TrimSpace(c.Query("days")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	counts, err := h.Auth.APIKeyUsageCountsSince(c.Request.Context(), BrowserPrincipal(c), since)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"since": since, "counts_by_key_id": counts})
}

func (h *AuthBFFHandler) MeProfile(c *gin.Context) {
	p := BrowserPrincipal(c)
	if p == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":  p.UserID,
		"username": p.Username,
		"role":     p.Role,
	})
}

func (h *AuthBFFHandler) MeListDevices(c *gin.Context) {
	rows, err := h.Auth.ListOwnDevices(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"devices": rows})
}

type patchAliasBody struct {
	Alias string `json:"alias"`
}

func (h *AuthBFFHandler) MePatchDeviceAlias(c *gin.Context) {
	var body patchAliasBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Auth.UpdateDeviceAlias(c.Request.Context(), BrowserPrincipal(c), c.Param("id"), body.Alias); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MeDeleteDevice(c *gin.Context) {
	if err := h.Auth.RevokeDeviceSession(c.Request.Context(), BrowserPrincipal(c), c.Param("id")); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MeRevokeAllSessions(c *gin.Context) {
	if err := h.Auth.RevokeAllOwnSessions(c.Request.Context(), BrowserPrincipal(c)); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", secure, true)
	c.SetCookie(deviceCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MePasskeyStatus(c *gin.Context) {
	st, err := h.Auth.PasskeyStatus(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": st})
}

func (h *AuthBFFHandler) MeListPasskeys(c *gin.Context) {
	rows, err := h.Auth.ListPasskeys(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": rows})
}

func (h *AuthBFFHandler) MeDeletePasskey(c *gin.Context) {
	if err := h.Auth.RevokePasskey(c.Request.Context(), BrowserPrincipal(c), c.Param("id")); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type passkeyBeginBody struct {
	DeviceName string `json:"device_name"`
}

func (h *AuthBFFHandler) MePasskeyRegistrationBegin(c *gin.Context) {
	var body passkeyBeginBody
	_ = c.ShouldBindJSON(&body)
	rpID, origin := bffWebAuthnRP(c)
	out, err := h.Auth.BeginPasskeyRegistration(c.Request.Context(), BrowserPrincipal(c), rpID, origin, body.DeviceName)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *AuthBFFHandler) MePasskeyRegistrationFinish(c *gin.Context) {
	var body any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Auth.FinishPasskeyRegistration(c.Request.Context(), BrowserPrincipal(c), body); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MePasskeyVerificationBegin(c *gin.Context) {
	rpID, origin := bffWebAuthnRP(c)
	out, err := h.Auth.BeginPasskeyVerification(c.Request.Context(), BrowserPrincipal(c), rpID, origin)
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *AuthBFFHandler) MePasskeyVerificationFinish(c *gin.Context) {
	var body any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Auth.FinishPasskeyVerification(c.Request.Context(), BrowserPrincipal(c), body); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MeListDeviceTokens(c *gin.Context) {
	rows, err := h.Auth.ListOwnDeviceTokens(c.Request.Context(), BrowserPrincipal(c))
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"device_tokens": rows})
}

type createDeviceTokenBody struct {
	DeviceName string `json:"device_name"`
}

func (h *AuthBFFHandler) MeCreateDeviceToken(c *gin.Context) {
	var body createDeviceTokenBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plain, err := h.Auth.CreateDeviceTokenForSession(c.Request.Context(), BrowserPrincipal(c), body.DeviceName, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		h.writeAuthErr(c, err)
		return
	}
	secure := requestShouldSetSecureCookie(c)
	dTTL := viper.GetDuration("auth.browser.device_token_ttl")
	if dTTL <= 0 {
		dTTL = 30 * 24 * time.Hour
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(deviceCookieName, plain, int(dTTL.Seconds()), "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MeRevokeDeviceToken(c *gin.Context) {
	if err := h.Auth.RevokeDeviceToken(c.Request.Context(), BrowserPrincipal(c), c.Param("id")); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(deviceCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) MeRevokeAllDeviceTokens(c *gin.Context) {
	if err := h.Auth.RevokeAllOwnDeviceTokens(c.Request.Context(), BrowserPrincipal(c)); err != nil {
		h.writeAuthErr(c, err)
		return
	}
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(deviceCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) writeAuthErr(c *gin.Context, err error) {
	switch err {
	case service.ErrAuthForbidden:
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case service.ErrAuthInvalidDeviceToken:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_device_token"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
