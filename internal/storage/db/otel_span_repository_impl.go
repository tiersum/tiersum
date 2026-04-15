package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/pkg/types"
)

// OtelSpanRepo persists OpenTelemetry span exports.
type OtelSpanRepo struct {
	db     sqlDB
	driver string
}

// NewOtelSpanRepo creates a span persistence repository.
func NewOtelSpanRepo(db sqlDB, driver string) *OtelSpanRepo {
	return &OtelSpanRepo{db: db, driver: driver}
}

// InsertSpan implements storage.IOtelSpanRepository.
func (r *OtelSpanRepo) InsertSpan(ctx context.Context, row *storage.OtelSpanRow) error {
	if row == nil {
		return nil
	}
	const sqliteQ = `
INSERT INTO otel_spans (trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano, status_code, status_message, attributes_json)
VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(trace_id, span_id) DO UPDATE SET
  end_time_unix_nano = excluded.end_time_unix_nano,
  status_code = excluded.status_code,
  status_message = excluded.status_message,
  attributes_json = excluded.attributes_json`
	const pgQ = `
INSERT INTO otel_spans (trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano, status_code, status_message, attributes_json)
VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (trace_id, span_id) DO UPDATE SET
  end_time_unix_nano = EXCLUDED.end_time_unix_nano,
  status_code = EXCLUDED.status_code,
  status_message = EXCLUDED.status_message,
  attributes_json = EXCLUDED.attributes_json`
	var err error
	if r.driver == "postgres" {
		_, err = r.db.ExecContext(ctx, pgQ, row.TraceID, row.SpanID, row.ParentSpanID, row.Name, row.Kind, row.StartUnixNano, row.EndUnixNano, row.StatusCode, row.StatusMessage, row.AttributesJSON)
	} else {
		_, err = r.db.ExecContext(ctx, sqliteQ, row.TraceID, row.SpanID, row.ParentSpanID, row.Name, row.Kind, row.StartUnixNano, row.EndUnixNano, row.StatusCode, row.StatusMessage, row.AttributesJSON)
	}
	return err
}

// ListTraceSummaries implements storage.IOtelSpanRepository.
func (r *OtelSpanRepo) ListTraceSummaries(ctx context.Context, limit, offset int) ([]types.OtelTraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	const sqliteQ = `
SELECT s.trace_id,
       MIN(s.start_time_unix_nano) AS t0,
       MAX(s.end_time_unix_nano) AS t1,
       COUNT(*) AS n,
       COALESCE((
         SELECT x.name FROM otel_spans x
         WHERE x.trace_id = s.trace_id
           AND (x.parent_span_id IS NULL OR x.parent_span_id = '')
         ORDER BY x.start_time_unix_nano ASC LIMIT 1
       ), '') AS root_name
FROM otel_spans s
GROUP BY s.trace_id
ORDER BY t0 DESC
LIMIT ? OFFSET ?`
	const pgQ = `
SELECT s.trace_id,
       MIN(s.start_time_unix_nano) AS t0,
       MAX(s.end_time_unix_nano) AS t1,
       COUNT(*)::int AS n,
       COALESCE((
         SELECT x.name FROM otel_spans x
         WHERE x.trace_id = s.trace_id
           AND (x.parent_span_id IS NULL OR x.parent_span_id = '')
         ORDER BY x.start_time_unix_nano ASC LIMIT 1
       ), '') AS root_name
FROM otel_spans s
GROUP BY s.trace_id
ORDER BY t0 DESC
LIMIT $1 OFFSET $2`
	var rows *sql.Rows
	var err error
	if r.driver == "postgres" {
		rows, err = r.db.QueryContext(ctx, pgQ, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, sqliteQ, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list trace summaries: %w", err)
	}
	defer rows.Close()
	var out []types.OtelTraceSummary
	for rows.Next() {
		var t types.OtelTraceSummary
		if err := rows.Scan(&t.TraceID, &t.StartedAtUnixNano, &t.EndedAtUnixNano, &t.SpanCount, &t.RootSpanName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListSpansByTraceID implements storage.IOtelSpanRepository.
func (r *OtelSpanRepo) ListSpansByTraceID(ctx context.Context, traceID string) ([]types.OtelSpanDTO, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("trace_id required")
	}
	const sqliteQ = `SELECT trace_id, span_id, COALESCE(parent_span_id, ''), name, kind, start_time_unix_nano, end_time_unix_nano, status_code, COALESCE(status_message, ''), COALESCE(attributes_json, '')
FROM otel_spans WHERE trace_id = ? ORDER BY start_time_unix_nano ASC`
	const pgQ = `SELECT trace_id, span_id, COALESCE(parent_span_id, ''), name, kind, start_time_unix_nano, end_time_unix_nano, status_code, COALESCE(status_message, ''), COALESCE(attributes_json, '')
FROM otel_spans WHERE trace_id = $1 ORDER BY start_time_unix_nano ASC`
	var rows *sql.Rows
	var err error
	if r.driver == "postgres" {
		rows, err = r.db.QueryContext(ctx, pgQ, traceID)
	} else {
		rows, err = r.db.QueryContext(ctx, sqliteQ, traceID)
	}
	if err != nil {
		return nil, fmt.Errorf("list spans: %w", err)
	}
	defer rows.Close()
	var out []types.OtelSpanDTO
	for rows.Next() {
		var s types.OtelSpanDTO
		if err := rows.Scan(&s.TraceID, &s.SpanID, &s.ParentSpanID, &s.Name, &s.Kind, &s.StartTimeUnixNano, &s.EndTimeUnixNano, &s.StatusCode, &s.StatusMessage, &s.AttributesJSON); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

var _ storage.IOtelSpanRepository = (*OtelSpanRepo)(nil)
