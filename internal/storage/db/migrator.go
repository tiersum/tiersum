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
