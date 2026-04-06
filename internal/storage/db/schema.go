// Package db provides database schema definitions
// Supports both SQLite (development) and PostgreSQL (production)
package db

// SchemaVersions holds all schema migrations
// Version numbers should be sequential
var SchemaVersions = []SchemaVersion{
	{
		Version:  1,
		Name:     "Initial schema",
		SQLite:   sqliteSchemaV1,
		Postgres: postgresSchemaV1,
	},
	{
		Version:  2,
		Name:     "Add topic summaries",
		SQLite:   sqliteSchemaV2,
		Postgres: postgresSchemaV2,
	},
	{
		Version:  3,
		Name:     "Add topic source",
		SQLite:   sqliteSchemaV3,
		Postgres: postgresSchemaV3,
	},
	{
		Version:  4,
		Name:     "Add summary hierarchy flags",
		SQLite:   sqliteSchemaV4,
		Postgres: postgresSchemaV4,
	},
}

// SchemaVersion represents a single schema migration
type SchemaVersion struct {
	Version  int
	Name     string
	SQLite   string
	Postgres string
}

// sqliteSchemaV1 - Initial schema for SQLite
const sqliteSchemaV1 = `
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

CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at);

-- Summaries table
CREATE TABLE IF NOT EXISTS summaries (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tier TEXT NOT NULL CHECK (tier IN ('topic', 'document', 'chapter', 'paragraph')),
    path TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_summaries_document_id ON summaries(document_id);
CREATE INDEX IF NOT EXISTS idx_summaries_tier ON summaries(tier);
CREATE INDEX IF NOT EXISTS idx_summaries_path ON summaries(path);
`

// postgresSchemaV1 - Initial schema for PostgreSQL
const postgresSchemaV1 = `
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

CREATE INDEX idx_documents_created_at ON documents(created_at);
CREATE INDEX idx_documents_tags ON documents USING GIN(tags);

-- Summaries table
CREATE TABLE IF NOT EXISTS summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tier VARCHAR(50) NOT NULL CHECK (tier IN ('topic', 'document', 'chapter', 'paragraph')),
    path VARCHAR(500) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_summaries_document_id ON summaries(document_id);
CREATE INDEX idx_summaries_tier ON summaries(tier);
CREATE INDEX idx_summaries_path ON summaries(path);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_summaries_updated_at
    BEFORE UPDATE ON summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
`

// sqliteSchemaV2 - Add topic summaries for SQLite
const sqliteSchemaV2 = `
-- Topic summaries table
CREATE TABLE IF NOT EXISTS topic_summaries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    summary TEXT NOT NULL,
    tags TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Junction table for topic-document relationships
CREATE TABLE IF NOT EXISTS topic_documents (
    topic_id TEXT NOT NULL REFERENCES topic_summaries(id) ON DELETE CASCADE,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (topic_id, document_id)
);

CREATE INDEX idx_topic_summaries_name ON topic_summaries(name);
CREATE INDEX idx_topic_documents_topic_id ON topic_documents(topic_id);
CREATE INDEX idx_topic_documents_document_id ON topic_documents(document_id);
`

// postgresSchemaV2 - Add topic summaries for PostgreSQL
const postgresSchemaV2 = `
-- Topic summaries table
CREATE TABLE IF NOT EXISTS topic_summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,
    summary TEXT NOT NULL,
    tags TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Junction table for topic-document relationships
CREATE TABLE IF NOT EXISTS topic_documents (
    topic_id UUID NOT NULL REFERENCES topic_summaries(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (topic_id, document_id)
);

CREATE INDEX idx_topic_summaries_name ON topic_summaries(name);
CREATE INDEX idx_topic_summaries_tags ON topic_summaries USING GIN(tags);
CREATE INDEX idx_topic_documents_topic_id ON topic_documents(topic_id);
CREATE INDEX idx_topic_documents_document_id ON topic_documents(document_id);

-- Update trigger for topic_summaries
CREATE TRIGGER update_topic_summaries_updated_at
    BEFORE UPDATE ON topic_summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
`

// sqliteSchemaV3 - Add topic source column for SQLite
const sqliteSchemaV3 = `
-- Add source column to topic_summaries table
ALTER TABLE topic_summaries ADD COLUMN source TEXT DEFAULT 'manual';

-- Update existing topics to have manual source
UPDATE topic_summaries SET source = 'manual' WHERE source IS NULL;
`

// postgresSchemaV3 - Add topic source column for PostgreSQL
const postgresSchemaV3 = `
-- Add source column to topic_summaries table
ALTER TABLE topic_summaries ADD COLUMN source VARCHAR(20) DEFAULT 'manual';

-- Update existing topics to have manual source
UPDATE topic_summaries SET source = 'manual' WHERE source IS NULL;
`

// sqliteSchemaV4 - Add is_source flag to summaries for SQLite
const sqliteSchemaV4 = `
-- Add is_source flag to summaries table
ALTER TABLE summaries ADD COLUMN is_source BOOLEAN DEFAULT 0;

-- Update existing summaries
UPDATE summaries SET is_source = 0 WHERE is_source IS NULL;
`

// postgresSchemaV4 - Add is_source flag to summaries for PostgreSQL
const postgresSchemaV4 = `
-- Add is_source flag to summaries table
ALTER TABLE summaries ADD COLUMN is_source BOOLEAN DEFAULT FALSE;

-- Update existing summaries
UPDATE summaries SET is_source = FALSE WHERE is_source IS NULL;
`

// GetSchemaForDriver returns schema for specific driver and version
func GetSchemaForDriver(driver string, version int) string {
	if version < 1 || version > len(SchemaVersions) {
		return ""
	}

	schema := SchemaVersions[version-1]
	if driver == "postgres" || driver == "postgresql" {
		return schema.Postgres
	}
	return schema.SQLite
}

// GetLatestVersion returns the latest schema version
func GetLatestVersion() int {
	return len(SchemaVersions)
}

// GetAllSchemas returns all schemas concatenated for a driver
func GetAllSchemas(driver string) string {
	var result string
	for i := 1; i <= len(SchemaVersions); i++ {
		result += GetSchemaForDriver(driver, i) + "\n"
	}
	return result
}
