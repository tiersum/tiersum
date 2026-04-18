package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func bffBrowserSessionMaxAgeSeconds() int {
	ttl := viper.GetDuration("auth.browser.session_ttl")
	if ttl <= 0 {
		ttl = 168 * time.Hour
	}
	return int(ttl.Seconds())
}

func bffDeviceTokenMaxAgeSeconds() int {
	dTTL := viper.GetDuration("auth.browser.device_token_ttl")
	if dTTL <= 0 {
		dTTL = 30 * 24 * time.Hour
	}
	return int(dTTL.Seconds())
}

func setBFFSessionCookie(c *gin.Context, sessionValue string) {
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, sessionValue, bffBrowserSessionMaxAgeSeconds(), "/", "", secure, true)
}

func setBFFDeviceCookie(c *gin.Context, deviceToken string) {
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(deviceCookieName, deviceToken, bffDeviceTokenMaxAgeSeconds(), "/", "", secure, true)
}

func clearBFFSessionAndDeviceCookies(c *gin.Context) {
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", secure, true)
	c.SetCookie(deviceCookieName, "", -1, "/", "", secure, true)
}

func clearBFFDeviceCookie(c *gin.Context) {
	secure := requestShouldSetSecureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(deviceCookieName, "", -1, "/", "", secure, true)
}
