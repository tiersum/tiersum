// Package db is the SQL persistence composition root for TierSum.
//
// Layout:
//   - shared — SQLDB contract, array/IN helpers, baseline DDL (BaseSchema)
//   - document — documents, chapters, catalog tags, topics
//   - auth — system_state, users, browser_sessions, api_keys, api_key_audit
//   - observability — OpenTelemetry span export rows
//
// NewUnitOfWork wires concrete repositories from these subpackages; call sites use *sql.DB as shared.SQLDB.
package db
