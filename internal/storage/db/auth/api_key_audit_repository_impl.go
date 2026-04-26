package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/tiersum/tiersum/internal/storage/db/shared"
)

// APIKeyAuditRepo implements storage.IAPIKeyAuditRepository.
type APIKeyAuditRepo struct {
	db     shared.SQLDB
	driver string
}

func NewAPIKeyAuditRepo(db shared.SQLDB, driver string) *APIKeyAuditRepo {
	return &APIKeyAuditRepo{db: db, driver: driver}
}

func (r *APIKeyAuditRepo) Insert(ctx context.Context, apiKeyID, method, path, clientIP string, at time.Time) error {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyAuditRepo.Insert")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, apiKeyID)
	shared.SetSpanInputString(span, "method", method)
	shared.SetSpanInputString(span, "path", path)
	shared.SetSpanInputString(span, "client_ip", clientIP)

	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"uuid", "", "", "", ""})
	q := fmt.Sprintf(`INSERT INTO api_key_audit (api_key_id, method, path, client_ip, called_at) VALUES (%s)`, vals)
	_, err := r.db.ExecContext(ctx, q, apiKeyID, method, path, clientIP, at)
	shared.SetSpanStatus(span, err)
	return err
}

func (r *APIKeyAuditRepo) CountSince(ctx context.Context, apiKeyID string, since time.Time) (int64, error) {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyAuditRepo.CountSince")
	if span != nil { defer span.End() }
	shared.SetSpanInputID(span, apiKeyID)

	ph1 := shared.Placeholder(r.driver, 1, "uuid")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`SELECT COUNT(*) FROM api_key_audit WHERE api_key_id = %s AND called_at >= %s`, ph1, ph2)
	var n int64
	err := r.db.QueryRowContext(ctx, q, apiKeyID, since).Scan(&n)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return 0, err
	}
	shared.SetSpanOutputCount(span, int(n))
	shared.SetSpanStatus(span, nil)
	return n, nil
}

func (r *APIKeyAuditRepo) CountsPerKeySince(ctx context.Context, since time.Time) (map[string]int64, error) {
	ctx, span := shared.WithRepoSpan(ctx, "APIKeyAuditRepo.CountsPerKeySince")
	if span != nil { defer span.End() }

	keyCol := "api_key_id"
	if shared.DriverIsPostgres(r.driver) {
		keyCol = "api_key_id::text"
	}
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s, COUNT(*) FROM api_key_audit WHERE called_at >= %s GROUP BY api_key_id`, keyCol, ph)
	rows, err := r.db.QueryContext(ctx, q, since)
	if err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var kid string
		var c int64
		if err := rows.Scan(&kid, &c); err != nil {
			shared.SetSpanStatus(span, err)
			return nil, err
		}
		out[kid] = c
	}
	if err := rows.Err(); err != nil {
		shared.SetSpanStatus(span, err)
		return nil, err
	}
	shared.SetSpanOutputCount(span, len(out))
	ids := make([]string, 0, len(out))
	for k := range out {
		ids = append(ids, k)
	}
	shared.SetSpanOutputIDs(span, ids)
	shared.SetSpanStatus(span, nil)
	return out, nil
}
