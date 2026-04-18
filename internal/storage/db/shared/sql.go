package shared

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// DriverIsPostgres reports whether the SQL dialect uses PostgreSQL-style placeholders ($n) and extensions.
func DriverIsPostgres(driver string) bool {
	d := strings.ToLower(strings.TrimSpace(driver))
	return d == "postgres" || d == "postgresql"
}

// Placeholder returns the positional placeholder for the n-th query argument (1-based).
// For PostgreSQL, pgCast (if non-empty) is appended as ::suffix (e.g. "uuid" -> "$1::uuid"); ignored on SQLite.
func Placeholder(driver string, index1Based int, pgCast string) string {
	if DriverIsPostgres(driver) {
		if pgCast != "" {
			return fmt.Sprintf("$%d::%s", index1Based, pgCast)
		}
		return fmt.Sprintf("$%d", index1Based)
	}
	return "?"
}

// PlaceholdersCSV returns a comma-separated list of n placeholders ("?, ?, ?" or "$1, $2, $3") without casts.
func PlaceholdersCSV(driver string, n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	if DriverIsPostgres(driver) {
		for i := 0; i < n; i++ {
			parts[i] = fmt.Sprintf("$%d", i+1)
		}
	} else {
		for i := 0; i < n; i++ {
			parts[i] = "?"
		}
	}
	return strings.Join(parts, ", ")
}

// PlaceholdersCSVWithPGCasts builds a comma-separated placeholder list; pgCasts[i] is an optional PostgreSQL ::cast suffix (ignored on SQLite).
func PlaceholdersCSVWithPGCasts(driver string, pgCasts []string) string {
	parts := make([]string, len(pgCasts))
	for i, c := range pgCasts {
		parts[i] = Placeholder(driver, i+1, c)
	}
	return strings.Join(parts, ", ")
}

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
	if DriverIsPostgres(driver) {
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
