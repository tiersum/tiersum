package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
)

// AuthBFFHandler registers human-track bootstrap, login, and admin JSON endpoints under /bff/v1.
type AuthBFFHandler struct {
	Auth          service.IAuthService
	Logger        *zap.Logger
	ServerVersion string
}

// NewAuthBFFHandler constructs browser-facing auth handlers.
func NewAuthBFFHandler(auth service.IAuthService, logger *zap.Logger, serverVersion string) *AuthBFFHandler {
	return &AuthBFFHandler{Auth: auth, Logger: logger, ServerVersion: strings.TrimSpace(serverVersion)}
}

// RegisterPublicRoutes mounts unauthenticated paths on the given group (must be /bff/v1).
func (h *AuthBFFHandler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/system/status", h.GetSystemStatus)
	rg.POST("/system/bootstrap", h.PostBootstrap)
	rg.POST("/auth/login", h.PostLogin)
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
}

// RegisterMeRoutes mounts /bff/v1/me/* (caller must apply BFFSessionMiddleware).
func (h *AuthBFFHandler) RegisterMeRoutes(rg *gin.RouterGroup) {
	rg.GET("/profile", h.MeProfile)
	rg.GET("/devices", h.MeListDevices)
	rg.PATCH("/devices/:id/alias", h.MePatchDeviceAlias)
	rg.DELETE("/devices/:id", h.MeDeleteDevice)
	rg.POST("/sessions/revoke_all", h.MeRevokeAllSessions)
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
}

func (h *AuthBFFHandler) PostLogin(c *gin.Context) {
	ctx := c.Request.Context()
	var body loginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sess, err := h.Auth.LoginWithAccessToken(ctx, body.AccessToken, body.Fingerprint, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
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
	// HttpOnly session cookie; Secure only when TLS (caller may terminate TLS upstream).
	secure := c.Request.TLS != nil
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
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthBFFHandler) writeAuthErr(c *gin.Context, err error) {
	switch err {
	case service.ErrAuthForbidden:
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
