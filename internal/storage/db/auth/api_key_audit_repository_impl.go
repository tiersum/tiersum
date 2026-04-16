package auth

import (
	"context"
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
	q := `INSERT INTO api_key_audit (api_key_id, method, path, client_ip, called_at) VALUES (?, ?, ?, ?, ?)`
	args := []interface{}{apiKeyID, method, path, clientIP, at}
	if r.driver == "postgres" {
		q = `INSERT INTO api_key_audit (api_key_id, method, path, client_ip, called_at) VALUES ($1::uuid, $2, $3, $4, $5)`
	}
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *APIKeyAuditRepo) CountSince(ctx context.Context, apiKeyID string, since time.Time) (int64, error) {
	q := `SELECT COUNT(*) FROM api_key_audit WHERE api_key_id = ? AND called_at >= ?`
	args := []interface{}{apiKeyID, since}
	if r.driver == "postgres" {
		q = `SELECT COUNT(*) FROM api_key_audit WHERE api_key_id = $1::uuid AND called_at >= $2`
	}
	var n int64
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func (r *APIKeyAuditRepo) CountsPerKeySince(ctx context.Context, since time.Time) (map[string]int64, error) {
	q := `SELECT api_key_id, COUNT(*) FROM api_key_audit WHERE called_at >= ? GROUP BY api_key_id`
	args := []interface{}{since}
	if r.driver == "postgres" {
		q = `SELECT api_key_id::text, COUNT(*) FROM api_key_audit WHERE called_at >= $1 GROUP BY api_key_id`
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
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
