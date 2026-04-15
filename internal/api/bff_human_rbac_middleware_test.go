package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

func TestBFFHumanRBAC_viewerAllowsGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/bff/v1/documents", nil)
	c.Set(ginKeyBrowserPrincipal, &service.BrowserPrincipal{Role: types.AuthRoleViewer})

	BFFHumanRBAC()(c)
	require.False(t, c.IsAborted())
}

func TestBFFHumanRBAC_viewerAllowsProgressivePOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/bff/v1/query/progressive", nil)
	c.Set(ginKeyBrowserPrincipal, &service.BrowserPrincipal{Role: types.AuthRoleViewer})

	BFFHumanRBAC()(c)
	require.False(t, c.IsAborted())
}

func TestBFFHumanRBAC_viewerBlocksIngestPOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/bff/v1/documents", nil)
	c.Set(ginKeyBrowserPrincipal, &service.BrowserPrincipal{Role: types.AuthRoleViewer})

	BFFHumanRBAC()(c)
	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestBFFHumanRBAC_userBlockedMonitoring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/bff/v1/monitoring", nil)
	c.Set(ginKeyBrowserPrincipal, &service.BrowserPrincipal{Role: types.AuthRoleUser})

	BFFHumanRBAC()(c)
	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestBFFHumanRBAC_adminAllowsMonitoring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/bff/v1/monitoring", nil)
	c.Set(ginKeyBrowserPrincipal, &service.BrowserPrincipal{Role: types.AuthRoleAdmin})

	BFFHumanRBAC()(c)
	require.False(t, c.IsAborted())
}
