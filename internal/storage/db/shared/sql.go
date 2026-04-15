package shared

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SQLDB is a minimal interface for database operations used by repository implementations.
type SQLDB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// ParseStringArray decodes a PostgreSQL/SQLite text array literal into a Go []string.
func ParseStringArray(s string) []string {
	if s == "" || s == "{}" {
		return []string{}
	}
	s = strings.Trim(s, "{}")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.Trim(p, "\"")
	}
	return result
}

// FormatStringArray converts a string slice to PostgreSQL array format.
func FormatStringArray(arr []string) string {
	if len(arr) == 0 {
		return "{}"
	}
	// Escape special characters and quote if needed
	parts := make([]string, len(arr))
	for i, s := range arr {
		// Escape backslashes and quotes
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		if strings.Contains(s, ",") || strings.Contains(s, "{") || strings.Contains(s, "}") {
			s = "\"" + s + "\""
		}
		parts[i] = s
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// BuildInPlaceholders returns a placeholder list and args for IN (...) clauses.
func BuildInPlaceholders(driver string, values []string) (string, []interface{}) {
	args := make([]interface{}, len(values))
	if driver == "postgres" {
		parts := make([]string, len(values))
		for i, v := range values {
			args[i] = v
			parts[i] = fmt.Sprintf("$%d", i+1)
		}
		return strings.Join(parts, ","), args
	}
	parts := make([]string, len(values))
	for i, v := range values {
		args[i] = v
		parts[i] = "?"
	}
	return strings.Join(parts, ","), args
}
