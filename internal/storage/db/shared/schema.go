// Package shared provides database schema definitions and SQL helpers.
// Supports both SQLite (development) and PostgreSQL (production).
package shared

// BaseSchema returns the latest base schema DDL for the given driver.
// This project starts from a clean database; migrations are introduced only after this baseline.
func BaseSchema(driver string) string {
	if DriverIsPostgres(driver) {
		return postgresBaseSchema
	}
	return sqliteBaseSchema
}

const sqliteBaseSchema = `
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'markdown',
    tags TEXT,
    status TEXT NOT NULL DEFAULT 'cold',
    hot_score REAL NOT NULL DEFAULT 0,
    query_count INTEGER NOT NULL DEFAULT 0,
    last_query_at DATETIME,
    embedding TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_hot_score ON documents(hot_score);
CREATE INDEX IF NOT EXISTS idx_documents_status_hot_score ON documents(status, hot_score);
CREATE INDEX IF NOT EXISTS idx_documents_status_created ON documents(status, created_at);

CREATE TABLE IF NOT EXISTS chapters (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chapters_document_id ON chapters(document_id);
CREATE INDEX IF NOT EXISTS idx_chapters_document_id_path ON chapters(document_id, path);

CREATE TABLE IF NOT EXISTS topics (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    tag_names TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);

CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    topic_id TEXT REFERENCES topics(id) ON DELETE SET NULL,
    document_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
CREATE INDEX IF NOT EXISTS idx_tags_topic_id ON tags(topic_id);

CREATE TABLE IF NOT EXISTS topic_regroup_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_count_before INTEGER,
    tag_count_after INTEGER,
    group_count INTEGER,
    duration_ms INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

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

CREATE TABLE IF NOT EXISTS system_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    initialized_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT OR IGNORE INTO system_state (id, initialized_at) VALUES (1, NULL);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user', 'viewer')),
    access_token_hash TEXT NOT NULL,
    token_expiry_mode TEXT NOT NULL DEFAULT 'slide' CHECK (token_expiry_mode IN ('slide', 'never')),
    max_devices INTEGER NOT NULL DEFAULT 3,
    token_valid_until DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_access_token_hash ON users(access_token_hash);

CREATE TABLE IF NOT EXISTS browser_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token_hash TEXT NOT NULL UNIQUE,
    fingerprint_hash TEXT NOT NULL,
    ip_prefix TEXT NOT NULL DEFAULT '',
    user_agent_norm TEXT NOT NULL DEFAULT '',
    timezone TEXT NOT NULL DEFAULT '',
    device_alias TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_browser_sessions_user_id ON browser_sessions(user_id);

CREATE TABLE IF NOT EXISTS device_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT '',
    ip_prefix TEXT NOT NULL DEFAULT '',
    user_agent_norm TEXT NOT NULL DEFAULT '',
    last_used_at DATETIME,
    expires_at DATETIME NOT NULL,
    revoked_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id ON device_tokens(user_id);

CREATE TABLE IF NOT EXISTS passkey_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id_b64 TEXT NOT NULL UNIQUE,
    public_key_b64 TEXT NOT NULL,
    sign_count INTEGER NOT NULL DEFAULT 0,
    device_name TEXT NOT NULL DEFAULT '',
    last_used_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_passkey_credentials_user_id ON passkey_credentials(user_id);

CREATE TABLE IF NOT EXISTS passkey_session_verifications (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    verified_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_passkey_session_verifications_user_id ON passkey_session_verifications(user_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('read', 'write', 'admin')),
    key_hash TEXT NOT NULL UNIQUE,
    revoked_at DATETIME,
    expires_at DATETIME,
    created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_used_at DATETIME,
    last_used_ip TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked ON api_keys(revoked_at);

CREATE TABLE IF NOT EXISTS api_key_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_key_id TEXT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    client_ip TEXT NOT NULL DEFAULT '',
    called_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_api_key_audit_key_time ON api_key_audit(api_key_id, called_at);
`

const postgresBaseSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(500) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    format VARCHAR(50) NOT NULL DEFAULT 'markdown',
    tags TEXT[],
    status VARCHAR(20) NOT NULL DEFAULT 'cold',
    hot_score REAL NOT NULL DEFAULT 0,
    query_count INTEGER NOT NULL DEFAULT 0,
    last_query_at TIMESTAMP WITH TIME ZONE,
    embedding REAL[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at);
CREATE INDEX IF NOT EXISTS idx_documents_tags ON documents USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_hot_score ON documents(hot_score);
CREATE INDEX IF NOT EXISTS idx_documents_status_hot_score ON documents(status, hot_score);
CREATE INDEX IF NOT EXISTS idx_documents_status_created ON documents(status, created_at);

CREATE TABLE IF NOT EXISTS chapters (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chapters_document_id ON chapters(document_id);
CREATE INDEX IF NOT EXISTS idx_chapters_document_id_path ON chapters(document_id, path);

CREATE TABLE IF NOT EXISTS topics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,
    tag_names TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);

CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    topic_id UUID REFERENCES topics(id) ON DELETE SET NULL,
    document_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
CREATE INDEX IF NOT EXISTS idx_tags_topic_id ON tags(topic_id);

CREATE TABLE IF NOT EXISTS topic_regroup_log (
    id SERIAL PRIMARY KEY,
    tag_count_before INTEGER,
    tag_count_after INTEGER,
    group_count INTEGER,
    duration_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

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

CREATE TABLE IF NOT EXISTS system_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    initialized_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO system_state (id, initialized_at) VALUES (1, NULL)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(200) NOT NULL UNIQUE,
    role VARCHAR(16) NOT NULL CHECK (role IN ('admin', 'user', 'viewer')),
    access_token_hash VARCHAR(64) NOT NULL,
    token_expiry_mode VARCHAR(16) NOT NULL DEFAULT 'slide' CHECK (token_expiry_mode IN ('slide', 'never')),
    max_devices INTEGER NOT NULL DEFAULT 3,
    token_valid_until TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_access_token_hash ON users(access_token_hash);

CREATE TABLE IF NOT EXISTS browser_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token_hash VARCHAR(64) NOT NULL UNIQUE,
    fingerprint_hash VARCHAR(64) NOT NULL,
    ip_prefix VARCHAR(64) NOT NULL DEFAULT '',
    user_agent_norm TEXT NOT NULL DEFAULT '',
    timezone VARCHAR(128) NOT NULL DEFAULT '',
    device_alias VARCHAR(200) NOT NULL DEFAULT '',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_browser_sessions_user_id ON browser_sessions(user_id);

CREATE TABLE IF NOT EXISTS device_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT '',
    ip_prefix TEXT NOT NULL DEFAULT '',
    user_agent_norm TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id ON device_tokens(user_id);

CREATE TABLE IF NOT EXISTS passkey_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id_b64 TEXT NOT NULL UNIQUE,
    public_key_b64 TEXT NOT NULL,
    sign_count BIGINT NOT NULL DEFAULT 0,
    device_name TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_passkey_credentials_user_id ON passkey_credentials(user_id);

CREATE TABLE IF NOT EXISTS passkey_session_verifications (
    session_id TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    verified_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_passkey_session_verifications_user_id ON passkey_session_verifications(user_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(300) NOT NULL,
    scope VARCHAR(16) NOT NULL CHECK (scope IN ('read', 'write', 'admin')),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    last_used_at TIMESTAMP WITH TIME ZONE,
    last_used_ip VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked ON api_keys(revoked_at);

CREATE TABLE IF NOT EXISTS api_key_audit (
    id BIGSERIAL PRIMARY KEY,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    method VARCHAR(16) NOT NULL,
    path TEXT NOT NULL,
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    called_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_api_key_audit_key_time ON api_key_audit(api_key_id, called_at);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
`
