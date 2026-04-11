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
	{
		Version:  5,
		Name:     "Add tag groups and remove topics",
		SQLite:   sqliteSchemaV5,
		Postgres: postgresSchemaV5,
	},
	{
		Version:  6,
		Name:     "Add hot/cold document tiering",
		SQLite:   sqliteSchemaV6,
		Postgres: postgresSchemaV6,
	},
	{
		Version:  7,
		Name:     "Add document embeddings for cold docs",
		SQLite:   sqliteSchemaV7,
		Postgres: postgresSchemaV7,
	},
	{
		Version:  8,
		Name:     "OpenTelemetry spans for progressive-query debug traces",
		SQLite:   sqliteSchemaV8,
		Postgres: postgresSchemaV8,
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

// sqliteSchemaV5 - Add tag groups, remove topics, update summaries tier enum
const sqliteSchemaV5 = `
-- Drop topic-related tables (data will be lost)
DROP TABLE IF EXISTS topic_documents;
DROP TABLE IF EXISTS topic_summaries;

-- Update summaries tier check constraint by recreating table
-- First create new table with correct schema
CREATE TABLE summaries_new (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tier TEXT NOT NULL CHECK (tier IN ('document', 'chapter', 'source')),
    path TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    is_source BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Copy data (paragraph tier entries will be converted to chapter)
INSERT INTO summaries_new (id, document_id, tier, path, content, is_source, created_at, updated_at)
SELECT id, document_id, 
    CASE WHEN tier IN ('topic', 'paragraph') THEN 'chapter' ELSE tier END,
    path, content, is_source, created_at, updated_at
FROM summaries;

-- Drop old table and rename new one
DROP TABLE summaries;
ALTER TABLE summaries_new RENAME TO summaries;

-- Recreate indexes
CREATE INDEX idx_summaries_document_id ON summaries(document_id);
CREATE INDEX idx_summaries_tier ON summaries(tier);
CREATE INDEX idx_summaries_path ON summaries(path);

-- Tag groups table (Level 1 categories)
CREATE TABLE IF NOT EXISTS tag_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    tags TEXT, -- JSON array of tags in this group
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tag_groups_name ON tag_groups(name);

-- Global tags table (Level 2 tags)
CREATE TABLE IF NOT EXISTS global_tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    group_id TEXT REFERENCES tag_groups(id) ON DELETE SET NULL,
    document_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_global_tags_name ON global_tags(name);
CREATE INDEX idx_global_tags_group_id ON global_tags(group_id);

-- Tag group refresh log (optional bookkeeping for grouping runs)
CREATE TABLE IF NOT EXISTS tag_group_refresh_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_count_before INTEGER,
    tag_count_after INTEGER,
    group_count INTEGER,
    duration_ms INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// postgresSchemaV5 - Add tag groups, remove topics, update summaries tier enum
const postgresSchemaV5 = `
-- Drop topic-related tables (data will be lost)
DROP TABLE IF EXISTS topic_documents;
DROP TABLE IF EXISTS topic_summaries;

-- Update summaries tier check constraint
-- First drop existing constraint (PostgreSQL specific)
ALTER TABLE summaries DROP CONSTRAINT IF EXISTS summaries_tier_check;

-- Add new check constraint
ALTER TABLE summaries ADD CONSTRAINT summaries_tier_check 
    CHECK (tier IN ('document', 'chapter', 'source'));

-- Update existing paragraph/topic entries to chapter
UPDATE summaries SET tier = 'chapter' WHERE tier IN ('paragraph', 'topic');

-- Tag groups table (Level 1 categories)
CREATE TABLE IF NOT EXISTS tag_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,
    tags TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tag_groups_name ON tag_groups(name);

-- Global tags table (Level 2 tags)
CREATE TABLE IF NOT EXISTS global_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    group_id UUID REFERENCES tag_groups(id) ON DELETE SET NULL,
    document_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_global_tags_name ON global_tags(name);
CREATE INDEX idx_global_tags_group_id ON global_tags(group_id);

-- Tag group refresh log
CREATE TABLE IF NOT EXISTS tag_group_refresh_log (
    id SERIAL PRIMARY KEY,
    tag_count_before INTEGER,
    tag_count_after INTEGER,
    group_count INTEGER,
    duration_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Triggers for updated_at
CREATE TRIGGER update_tag_groups_updated_at
    BEFORE UPDATE ON tag_groups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_global_tags_updated_at
    BEFORE UPDATE ON global_tags
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
`

// sqliteSchemaV6 - Add hot/cold document tiering for SQLite
const sqliteSchemaV6 = `
-- Add hot/cold tiering columns to documents table
ALTER TABLE documents ADD COLUMN status TEXT DEFAULT 'cold';
ALTER TABLE documents ADD COLUMN hot_score REAL DEFAULT 0;
ALTER TABLE documents ADD COLUMN query_count INTEGER DEFAULT 0;
ALTER TABLE documents ADD COLUMN last_query_at DATETIME;

-- Create indexes for hot/cold tiering queries
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_hot_score ON documents(hot_score);
CREATE INDEX IF NOT EXISTS idx_documents_status_hot_score ON documents(status, hot_score);
`

// postgresSchemaV6 - Add hot/cold document tiering for PostgreSQL
const postgresSchemaV6 = `
-- Add hot/cold tiering columns to documents table
ALTER TABLE documents ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'cold';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS hot_score REAL DEFAULT 0;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS query_count INTEGER DEFAULT 0;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS last_query_at TIMESTAMP WITH TIME ZONE;

-- Create indexes for hot/cold tiering queries
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_hot_score ON documents(hot_score);
CREATE INDEX IF NOT EXISTS idx_documents_status_hot_score ON documents(status, hot_score);
`

// sqliteSchemaV7 - Add document embeddings for cold docs (SQLite)
const sqliteSchemaV7 = `
-- Add embedding column to documents table for vector search
-- Stored as JSON array of floats (384 dimensions for MiniLM-L6-v2)
ALTER TABLE documents ADD COLUMN embedding TEXT;

-- Create index for status-based queries (used when loading cold docs on startup)
CREATE INDEX IF NOT EXISTS idx_documents_status_created ON documents(status, created_at);
`

// postgresSchemaV7 - Add document embeddings for cold docs (PostgreSQL)
const postgresSchemaV7 = `
-- Add embedding column to documents table for vector search
-- Using PostgreSQL array type for vector storage
ALTER TABLE documents ADD COLUMN IF NOT EXISTS embedding REAL[];

-- Create index for status-based queries
CREATE INDEX IF NOT EXISTS idx_documents_status_created ON documents(status, created_at);

-- Optional: Create IVFFlat index for vector similarity search if pgvector extension is available
-- Note: This requires the pgvector extension to be installed
-- CREATE INDEX IF NOT EXISTS idx_documents_embedding ON documents USING ivfflat (embedding vector_cosine_ops);
`

// sqliteSchemaV8 stores exported OpenTelemetry spans (progressive query debug only).
const sqliteSchemaV8 = `
CREATE TABLE IF NOT EXISTS otel_spans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    parent_span_id TEXT,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    start_time_unix_nano INTEGER NOT NULL,
    end_time_unix_nano INTEGER NOT NULL,
    status_code TEXT NOT NULL,
    status_message TEXT,
    attributes_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(trace_id, span_id)
);
CREATE INDEX IF NOT EXISTS idx_otel_spans_trace_id ON otel_spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_otel_spans_created_at ON otel_spans(created_at);
`

// postgresSchemaV8 stores exported OpenTelemetry spans.
const postgresSchemaV8 = `
CREATE TABLE IF NOT EXISTS otel_spans (
    id BIGSERIAL PRIMARY KEY,
    trace_id VARCHAR(32) NOT NULL,
    span_id VARCHAR(16) NOT NULL,
    parent_span_id VARCHAR(16),
    name TEXT NOT NULL,
    kind VARCHAR(32) NOT NULL,
    start_time_unix_nano BIGINT NOT NULL,
    end_time_unix_nano BIGINT NOT NULL,
    status_code VARCHAR(16) NOT NULL,
    status_message TEXT,
    attributes_json TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(trace_id, span_id)
);
CREATE INDEX IF NOT EXISTS idx_otel_spans_trace_id ON otel_spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_otel_spans_created_at ON otel_spans(created_at);
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
