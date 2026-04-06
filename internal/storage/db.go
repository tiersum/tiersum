package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

// DB wraps sql.DB with driver information
type DB struct {
	*sql.DB
	driver string
}

// NewDB creates a new database connection based on configuration
func NewDB() (*DB, error) {
	driver := viper.GetString("storage.database.driver")
	if driver == "" {
		driver = "sqlite3" // default to SQLite
	}

	switch driver {
	case "sqlite3", "sqlite":
		return newSQLiteDB()
	case "postgres", "postgresql":
		return newPostgresDB()
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// newSQLiteDB creates a new SQLite database connection
func newSQLiteDB() (*DB, error) {
	dsn := viper.GetString("storage.database.dsn")
	if dsn == "" {
		dsn = "./data/tiersum.db" // default SQLite path
	}

	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// Set connection pool for SQLite
	db.SetMaxOpenConns(1) // SQLite recommends single writer
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	return &DB{DB: db, driver: "sqlite3"}, nil
}

// newPostgresDB creates a new PostgreSQL database connection
func newPostgresDB() (*DB, error) {
	dsn := viper.GetString("storage.database.dsn")
	if dsn == "" {
		return nil, fmt.Errorf("postgres DSN not configured")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres db: %w", err)
	}

	// Set connection pool
	maxConns := viper.GetInt("storage.database.max_connections")
	if maxConns == 0 {
		maxConns = 20
	}
	minConns := viper.GetInt("storage.database.min_connections")
	if minConns == 0 {
		minConns = 5
	}

	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(minConns)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres db: %w", err)
	}

	return &DB{DB: db, driver: "postgres"}, nil
}

// Driver returns the database driver name
func (db *DB) Driver() string {
	return db.driver
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// Health checks if database is healthy
func (db *DB) Health(ctx context.Context) error {
	return db.PingContext(ctx)
}

// InitSchema initializes the database schema
func (db *DB) InitSchema() error {
	if db.driver == "sqlite3" {
		return initSQLiteSchema(db.DB)
	}
	return initPostgresSchema(db.DB)
}

// initSQLiteSchema initializes SQLite schema
func initSQLiteSchema(db *sql.DB) error {
	schema := `
-- Documents table
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'markdown',
    tags TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Summaries table
CREATE TABLE IF NOT EXISTS summaries (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tier TEXT NOT NULL CHECK (tier IN ('document', 'chapter', 'paragraph')),
    path TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at);
CREATE INDEX IF NOT EXISTS idx_summaries_document_id ON summaries(document_id);
CREATE INDEX IF NOT EXISTS idx_summaries_tier ON summaries(tier);
CREATE INDEX IF NOT EXISTS idx_summaries_path ON summaries(path);
`
	_, err := db.Exec(schema)
	return err
}

// initPostgresSchema initializes PostgreSQL schema
func initPostgresSchema(db *sql.DB) error {
	schema := `
-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Documents table
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    format VARCHAR(50) NOT NULL DEFAULT 'markdown',
    tags TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Summaries table
CREATE TABLE IF NOT EXISTS summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tier VARCHAR(50) NOT NULL CHECK (tier IN ('document', 'chapter', 'paragraph')),
    path VARCHAR(500) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at);
CREATE INDEX IF NOT EXISTS idx_documents_tags ON documents USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_summaries_document_id ON summaries(document_id);
CREATE INDEX IF NOT EXISTS idx_summaries_tier ON summaries(tier);
CREATE INDEX IF NOT EXISTS idx_summaries_path ON summaries(path);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
DROP TRIGGER IF EXISTS update_documents_updated_at ON documents;
CREATE TRIGGER update_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_summaries_updated_at ON summaries;
CREATE TRIGGER update_summaries_updated_at
    BEFORE UPDATE ON summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
`
	_, err := db.Exec(schema)
	return err
}
