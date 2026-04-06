-- Migration: Add topic summaries and enhance summary tiers
-- Adds support for topic-level summaries and extends tier enum

-- Update summaries table to support additional tiers
-- Note: The CHECK constraint will need to be updated in application layer for SQLite
-- For PostgreSQL, we can use ALTER TABLE

-- Create topic_summaries table
CREATE TABLE IF NOT EXISTS topic_summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,
    summary TEXT NOT NULL,
    tags TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create junction table for topic-document relationships
CREATE TABLE IF NOT EXISTS topic_documents (
    topic_id UUID NOT NULL REFERENCES topic_summaries(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (topic_id, document_id)
);

-- Create indexes
CREATE INDEX idx_topic_summaries_name ON topic_summaries(name);
CREATE INDEX idx_topic_summaries_tags ON topic_summaries USING GIN(tags);
CREATE INDEX idx_topic_documents_topic_id ON topic_documents(topic_id);
CREATE INDEX idx_topic_documents_document_id ON topic_documents(document_id);

-- Update trigger for topic_summaries
CREATE TRIGGER update_topic_summaries_updated_at
    BEFORE UPDATE ON topic_summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- For SQLite compatibility, we need to handle the tier constraint differently
-- SQLite doesn't support ALTER TABLE for CHECK constraints easily
-- The application will validate tier values
