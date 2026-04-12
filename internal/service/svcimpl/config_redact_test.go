package svcimpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

func TestRedactConfigMap_nestedSecrets(t *testing.T) {
	in := map[string]interface{}{
		"llm": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key":     "sk-secret",
				"model":       "gpt-4o-mini",
				"temperature": 0.3,
			},
		},
		"storage": map[string]interface{}{
			"database": map[string]interface{}{
				"driver": "sqlite3",
				"dsn":    "./data/secret.db",
			},
		},
	}
	out := redactConfigMap(in)
	llm := out["llm"].(map[string]interface{})
	openai := llm["openai"].(map[string]interface{})
	assert.Contains(t, openai["api_key"], redactedPlaceholder)
	assert.Equal(t, "gpt-4o-mini", openai["model"])
	st := out["storage"].(map[string]interface{})
	db := st["database"].(map[string]interface{})
	assert.Equal(t, "sqlite3", db["driver"])
	assert.Contains(t, db["dsn"], redactedPlaceholder)
}

func TestRedactConfigMap_allowlistMaxTokens(t *testing.T) {
	in := map[string]interface{}{
		"llm": map[string]interface{}{
			"openai": map[string]interface{}{
				"max_tokens": 2000,
				"api_key":    "x",
			},
		},
	}
	out := redactConfigMap(in)
	openai := out["llm"].(map[string]interface{})["openai"].(map[string]interface{})
	assert.EqualValues(t, 2000, openai["max_tokens"])
	assert.Contains(t, openai["api_key"], redactedPlaceholder)
}

func TestRedactStringLeaf_jwt(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	got := redactStringLeaf(jwt)
	assert.Contains(t, got, "jwt")
}

func TestRedactStringLeaf_skPrefix(t *testing.T) {
	got := redactStringLeaf("sk-live-abcdefghijklmnopqrstuvwxyz")
	assert.Contains(t, got, "credential-prefix")
}

func TestRedactStringLeaf_urlUserinfo(t *testing.T) {
	in := "postgres://dbuser:dbsecret@localhost:5432/mydb?sslmode=disable"
	got := redactStringLeaf(in)
	assert.True(t, strings.Contains(got, "*:*@"))
	assert.False(t, strings.Contains(got, "dbsecret"))
}

func TestRedactConfigMap_embeddedSecretInNonSensitiveKey(t *testing.T) {
	in := map[string]interface{}{
		"docs": map[string]interface{}{
			"example_jwt": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		},
	}
	out := redactConfigMap(in)
	ex := out["docs"].(map[string]interface{})["example_jwt"].(string)
	assert.Contains(t, ex, "jwt")
}

func TestAdminConfigViewSvc_RedactedSnapshot_requiresAdmin(t *testing.T) {
	s := NewAdminConfigViewSvc()
	_, err := s.RedactedSnapshot(t.Context(), &service.BrowserPrincipal{Role: types.AuthRoleUser})
	require.ErrorIs(t, err, service.ErrAuthForbidden)
}
