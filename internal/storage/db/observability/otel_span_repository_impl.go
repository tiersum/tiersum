package observability

import (
	"context"
	"fmt"
	"strings"

	"github.com/tiersum/tiersum/internal/storage"
	"github.com/tiersum/tiersum/internal/storage/db/shared"
	"github.com/tiersum/tiersum/pkg/types"
)

// OtelSpanRepo persists OpenTelemetry span exports.
type OtelSpanRepo struct {
	db     shared.SQLDB
	driver string
}

// NewOtelSpanRepo creates a span persistence repository.
func NewOtelSpanRepo(db shared.SQLDB, driver string) *OtelSpanRepo {
	return &OtelSpanRepo{db: db, driver: driver}
}

// InsertSpan implements storage.IOtelSpanRepository.
func (r *OtelSpanRepo) InsertSpan(ctx context.Context, row *storage.OtelSpanRow) error {
	if row == nil {
		return nil
	}
	ph := func(n int) string { return shared.Placeholder(r.driver, n, "") }
	vals := fmt.Sprintf("%s, %s, NULLIF(%s, ''), %s, %s, %s, %s, %s, %s, %s",
		ph(1), ph(2), ph(3), ph(4), ph(5), ph(6), ph(7), ph(8), ph(9), ph(10))
	var conflictTail string
	if shared.DriverIsPostgres(r.driver) {
		conflictTail = `
ON CONFLICT (trace_id, span_id) DO UPDATE SET
  end_time_unix_nano = EXCLUDED.end_time_unix_nano,
  status_code = EXCLUDED.status_code,
  status_message = EXCLUDED.status_message,
  attributes_json = EXCLUDED.attributes_json`
	} else {
		conflictTail = `
ON CONFLICT(trace_id, span_id) DO UPDATE SET
  end_time_unix_nano = excluded.end_time_unix_nano,
  status_code = excluded.status_code,
  status_message = excluded.status_message,
  attributes_json = excluded.attributes_json`
	}
	q := fmt.Sprintf(`
INSERT INTO otel_spans (trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano, status_code, status_message, attributes_json)
VALUES (%s)%s`, vals, conflictTail)
	_, err := r.db.ExecContext(ctx, q, row.TraceID, row.SpanID, row.ParentSpanID, row.Name, row.Kind, row.StartUnixNano, row.EndUnixNano, row.StatusCode, row.StatusMessage, row.AttributesJSON)
	return err
}

// ListTraceSummaries implements storage.IOtelSpanRepository.
func (r *OtelSpanRepo) ListTraceSummaries(ctx context.Context, serviceName string, limit, offset int) ([]types.OtelTraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	serviceName = strings.TrimSpace(serviceName)
	pat := fmt.Sprintf(`%%"service.name":"%s"%%`, strings.ReplaceAll(serviceName, `"`, `\"`))

	countExpr := "COUNT(*)"
	if shared.DriverIsPostgres(r.driver) {
		countExpr = "COUNT(*)::int"
	}
	ph1 := shared.Placeholder(r.driver, 1, "")
	ph2 := shared.Placeholder(r.driver, 2, "")
	ph3 := shared.Placeholder(r.driver, 3, "")
	q := fmt.Sprintf(`
WITH svc AS (
  SELECT * FROM otel_spans
  WHERE COALESCE(attributes_json, '') LIKE %s
)
SELECT s.trace_id,
       MIN(s.start_time_unix_nano) AS t0,
       MAX(s.end_time_unix_nano) AS t1,
       %s AS n,
       COALESCE((
         SELECT x.name FROM svc x
         WHERE x.trace_id = s.trace_id
           AND (x.parent_span_id IS NULL OR x.parent_span_id = '')
         ORDER BY x.start_time_unix_nano ASC LIMIT 1
       ), '') AS root_name
FROM svc s
WHERE EXISTS (
  SELECT 1 FROM svc r
  WHERE r.trace_id = s.trace_id
    AND (r.parent_span_id IS NULL OR r.parent_span_id = '')
)
GROUP BY s.trace_id
ORDER BY t0 DESC
LIMIT %s OFFSET %s`, ph1, countExpr, ph2, ph3)

	rows, err := r.db.QueryContext(ctx, q, pat, limit, offset)
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
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT trace_id, span_id, COALESCE(parent_span_id, ''), name, kind, start_time_unix_nano, end_time_unix_nano, status_code, COALESCE(status_message, ''), COALESCE(attributes_json, '')
FROM otel_spans WHERE trace_id = %s ORDER BY start_time_unix_nano ASC`, ph)
	rows, err := r.db.QueryContext(ctx, q, traceID)
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
