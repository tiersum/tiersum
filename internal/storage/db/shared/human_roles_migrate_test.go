package shared

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestMigrateHumanBrowserRolesSQLiteAllowsViewer(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    access_token_hash TEXT NOT NULL,
    token_expiry_mode TEXT NOT NULL DEFAULT 'slide' CHECK (token_expiry_mode IN ('slide', 'never')),
    max_devices INTEGER NOT NULL DEFAULT 3,
    token_valid_until DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`)
	require.NoError(t, err)

	require.NoError(t, MigrateHumanBrowserRoles(ctx, db, "sqlite3"))

	_, err = db.ExecContext(ctx, `INSERT INTO users (id, username, role, access_token_hash) VALUES ('a', 'alice', 'viewer', 'hash')`)
	require.NoError(t, err, "viewer insert should succeed after CHECK migration")

	require.NoError(t, MigrateHumanBrowserRoles(ctx, db, "sqlite3"), "second migration should be a no-op")
}
