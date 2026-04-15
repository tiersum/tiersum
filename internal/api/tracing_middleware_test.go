package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
)

func TestHTTPSpanStatusFromCode(t *testing.T) {
	cases := []struct {
		code     int
		wantCode codes.Code
		errSub   string // substring of description when Error
	}{
		{0, codes.Ok, ""},
		{http.StatusContinue, codes.Ok, ""},
		{http.StatusOK, codes.Ok, ""},
		{http.StatusCreated, codes.Ok, ""},
		{http.StatusFound, codes.Ok, ""},
		{http.StatusNotModified, codes.Ok, ""},
		{http.StatusBadRequest, codes.Error, "HTTP 400"},
		{http.StatusUnauthorized, codes.Error, "HTTP 401"},
		{http.StatusForbidden, codes.Error, "HTTP 403"},
		{http.StatusNotFound, codes.Error, "HTTP 404"},
		{http.StatusConflict, codes.Error, "HTTP 409"},
		{http.StatusUnprocessableEntity, codes.Error, "HTTP 422"},
		{http.StatusInternalServerError, codes.Error, "HTTP 500"},
		{http.StatusBadGateway, codes.Error, "HTTP 502"},
		{999, codes.Error, "HTTP 999"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("code_%d", tc.code), func(t *testing.T) {
			gotCode, gotDesc := httpSpanStatusFromResponseCode(tc.code)
			require.Equal(t, tc.wantCode, gotCode)
			if tc.wantCode == codes.Error {
				require.Contains(t, gotDesc, tc.errSub)
			} else {
				require.Equal(t, "", gotDesc)
			}
		})
	}
}
