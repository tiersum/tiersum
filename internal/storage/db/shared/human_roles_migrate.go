package shared

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var pgSafeIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// MigrateHumanBrowserRoles relaxes legacy users.role CHECK constraints to allow the viewer role.
// Safe to run on every startup (no-op when already migrated).
func MigrateHumanBrowserRoles(ctx context.Context, db *sql.DB, driver string) error {
	switch driver {
	case "sqlite3":
		return migrateHumanRolesSQLite(ctx, db)
	case "postgres":
		return migrateHumanRolesPostgres(ctx, db)
	default:
		return nil
	}
}

func migrateHumanRolesSQLite(ctx context.Context, db *sql.DB) error {
	var ddl string
	err := db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&ddl)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate human roles (sqlite): read sqlite_master: %w", err)
	}
	if strings.Contains(ddl, "'viewer'") {
		return nil
	}

	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): pragma off: %w", err)
	}
	defer func() { _, _ = db.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`) }()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate human roles (sqlite): begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	const createNew = `
CREATE TABLE users_new (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user', 'viewer')),
    access_token_hash TEXT NOT NULL,
    token_expiry_mode TEXT NOT NULL DEFAULT 'slide' CHECK (token_expiry_mode IN ('slide', 'never')),
    max_devices INTEGER NOT NULL DEFAULT 3,
    token_valid_until DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`
	if _, err := tx.ExecContext(ctx, createNew); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): create users_new: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO users_new (id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at)
SELECT id, username, role, access_token_hash, token_expiry_mode, max_devices, token_valid_until, created_at, updated_at FROM users`); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): copy rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE users`); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): drop users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE users_new RENAME TO users`); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): rename: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_users_access_token_hash ON users(access_token_hash)`); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): recreate index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate human roles (sqlite): commit: %w", err)
	}
	return nil
}

func migrateHumanRolesPostgres(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema = current_schema() AND table_name = 'users'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate human roles (postgres): table check: %w", err)
	}
	if n == 0 {
		return nil
	}

	rows, err := db.QueryContext(ctx, `
SELECT c.conname, pg_get_constraintdef(c.oid) AS def
FROM pg_constraint c
JOIN pg_class t ON t.oid = c.conrelid
JOIN pg_namespace n ON n.oid = c.connamespace
WHERE t.relname = 'users' AND n.nspname = current_schema() AND c.contype = 'c'`)
	if err != nil {
		return fmt.Errorf("migrate human roles (postgres): list checks: %w", err)
	}
	defer rows.Close()

	type pair struct{ name, def string }
	var all []pair
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			return fmt.Errorf("migrate human roles (postgres): scan: %w", err)
		}
		all = append(all, pair{name: name, def: def})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.def), "viewer") {
			return nil
		}
	}
	var toDrop []pair
	for _, p := range all {
		low := strings.ToLower(p.def)
		// Legacy human role enum: CHECK (role IN (...)) on column "role" only.
		if strings.Contains(low, "token_expiry") {
			continue
		}
		if strings.Contains(low, "(role") && strings.Contains(low, "admin") && strings.Contains(low, "user") {
			toDrop = append(toDrop, p)
		}
	}
	if len(toDrop) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate human roles (postgres): begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, p := range toDrop {
		if !pgSafeIdent.MatchString(p.name) {
			return fmt.Errorf("migrate human roles (postgres): unexpected constraint name %q", p.name)
		}
		q := fmt.Sprintf(`ALTER TABLE users DROP CONSTRAINT %q`, p.name)
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrate human roles (postgres): drop %q: %w", p.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user', 'viewer'))`); err != nil {
		return fmt.Errorf("migrate human roles (postgres): add check: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate human roles (postgres): commit: %w", err)
	}
	return nil
}
