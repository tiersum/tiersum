package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Migrator handles database schema migrations
type Migrator struct {
	db     *sql.DB
	driver string
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB, driver string) *Migrator {
	return &Migrator{
		db:     db,
		driver: driver,
	}
}

// InitSchema initializes the database schema to latest version
func (m *Migrator) InitSchema(ctx context.Context) error {
	// For SQLite, we can simply execute all schemas
	// For PostgreSQL, we should track versions in a migrations table

	schema := GetAllSchemas(m.driver)

	_, err := m.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	return nil
}

// GetCurrentVersion returns current schema version
// Returns 0 if no migrations table exists
func (m *Migrator) GetCurrentVersion(ctx context.Context) (int, error) {
	// Check if migrations table exists
	var exists bool
	var query string

	if m.driver == "postgres" {
		query = `SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_migrations')`
	} else {
		query = `SELECT COUNT(*) > 0 FROM sqlite_master WHERE type='table' AND name='schema_migrations'`
	}

	err := m.db.QueryRowContext(ctx, query).Scan(&exists)
	if err != nil || !exists {
		return 0, nil // No migrations table, assume fresh database
	}

	var version int
	err = m.db.QueryRowContext(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err != nil {
		return 0, nil
	}

	return version, nil
}

// MigrateUp runs all pending migrations with context
func (m *Migrator) MigrateUp(ctx context.Context) error {
	current, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return err
	}

	latest := GetLatestVersion()
	if current >= latest {
		return nil // Already up to date
	}

	// Run pending migrations
	for i := current + 1; i <= latest; i++ {
		schema := GetSchemaForDriver(m.driver, i)
		if schema == "" {
			continue
		}

		_, err := m.db.ExecContext(ctx, schema)
		if err != nil {
			return fmt.Errorf("migrate to version %d: %w", i, err)
		}
	}

	return nil
}

// MigrateUp runs all pending migrations (convenience method without context param)
func (m *Migrator) MigrateUpSimple() error {
	ctx := context.Background()
	return m.MigrateUp(ctx)
}

// EnsureAuthTables applies schema version 9 (dual-track auth) when core tables are missing.
// Like EnsureOtelSpansTable, this guards against MigrateUp errors being ignored in cmd/main.go
// ("continuing anyway") or partial application, so /bff/v1/system/status does not fail on a
// cold database that already has older tiersum tables.
func (m *Migrator) EnsureAuthTables(ctx context.Context) error {
	ok, err := m.tableExists(ctx, "system_state")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	schema := GetSchemaForDriver(m.driver, 9)
	if schema == "" {
		return fmt.Errorf("ensure auth tables: no schema for driver %q at version 9", m.driver)
	}
	_, err = m.db.ExecContext(ctx, schema)
	return err
}

// EnsureOtelSpansTable creates the otel_spans table and indexes when missing.
// It is independent of schema_migrations: MigrateUp may skip version 8 if version
// tracking is ahead of applied DDL, or fail before reaching v8 while the server
// still runs ("continuing anyway").
func (m *Migrator) EnsureOtelSpansTable(ctx context.Context) error {
	ok, err := m.tableExists(ctx, "otel_spans")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	schema := GetSchemaForDriver(m.driver, 8)
	if schema == "" {
		return fmt.Errorf("ensure otel_spans: no schema for driver %q at version 8", m.driver)
	}
	_, err = m.db.ExecContext(ctx, schema)
	return err
}

func (m *Migrator) tableExists(ctx context.Context, name string) (bool, error) {
	if m.driver == "postgres" || m.driver == "postgresql" {
		var n int
		err := m.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`,
			name,
		).Scan(&n)
		if err != nil {
			return false, err
		}
		return n > 0, nil
	}
	var n int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
		name,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// EnsureMigrationsTable creates the migrations tracking table
func (m *Migrator) EnsureMigrationsTable(ctx context.Context) error {
	var query string

	if m.driver == "postgres" {
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			)
		`
	} else {
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`
	}

	_, err := m.db.ExecContext(ctx, query)
	return err
}
