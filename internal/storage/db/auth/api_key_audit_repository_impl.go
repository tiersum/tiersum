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
	vals := shared.PlaceholdersCSVWithPGCasts(r.driver, []string{"uuid", "", "", "", ""})
	q := fmt.Sprintf(`INSERT INTO api_key_audit (api_key_id, method, path, client_ip, called_at) VALUES (%s)`, vals)
	_, err := r.db.ExecContext(ctx, q, apiKeyID, method, path, clientIP, at)
	return err
}

func (r *APIKeyAuditRepo) CountSince(ctx context.Context, apiKeyID string, since time.Time) (int64, error) {
	ph1 := shared.Placeholder(r.driver, 1, "uuid")
	ph2 := shared.Placeholder(r.driver, 2, "")
	q := fmt.Sprintf(`SELECT COUNT(*) FROM api_key_audit WHERE api_key_id = %s AND called_at >= %s`, ph1, ph2)
	var n int64
	err := r.db.QueryRowContext(ctx, q, apiKeyID, since).Scan(&n)
	return n, err
}

func (r *APIKeyAuditRepo) CountsPerKeySince(ctx context.Context, since time.Time) (map[string]int64, error) {
	keyCol := "api_key_id"
	if shared.DriverIsPostgres(r.driver) {
		keyCol = "api_key_id::text"
	}
	ph := shared.Placeholder(r.driver, 1, "")
	q := fmt.Sprintf(`SELECT %s, COUNT(*) FROM api_key_audit WHERE called_at >= %s GROUP BY api_key_id`, keyCol, ph)
	rows, err := r.db.QueryContext(ctx, q, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var kid string
		var c int64
		if err := rows.Scan(&kid, &c); err != nil {
			return nil, err
		}
		out[kid] = c
	}
	return out, rows.Err()
}
