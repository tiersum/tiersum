package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDriverIsPostgres(t *testing.T) {
	assert.True(t, DriverIsPostgres("postgres"))
	assert.True(t, DriverIsPostgres("PostgreSQL"))
	assert.True(t, DriverIsPostgres(" postgresql "))
	assert.False(t, DriverIsPostgres("sqlite3"))
	assert.False(t, DriverIsPostgres(""))
}

func TestPlaceholder(t *testing.T) {
	assert.Equal(t, "?", Placeholder("sqlite3", 1, "uuid"))
	assert.Equal(t, "?", Placeholder("sqlite3", 2, ""))
	assert.Equal(t, "$1", Placeholder("postgres", 1, ""))
	assert.Equal(t, "$2::uuid", Placeholder("postgres", 2, "uuid"))
}

func TestPlaceholdersCSV(t *testing.T) {
	assert.Equal(t, "", PlaceholdersCSV("sqlite3", 0))
	assert.Equal(t, "?", PlaceholdersCSV("sqlite3", 1))
	assert.Equal(t, "?, ?", PlaceholdersCSV("sqlite3", 2))
	assert.Equal(t, "$1", PlaceholdersCSV("postgres", 1))
	assert.Equal(t, "$1, $2, $3", PlaceholdersCSV("postgres", 3))
}

func TestPlaceholdersCSVWithPGCasts(t *testing.T) {
	got := PlaceholdersCSVWithPGCasts("sqlite3", []string{"uuid", "", "uuid"})
	assert.Equal(t, "?, ?, ?", got)
	gotPg := PlaceholdersCSVWithPGCasts("postgres", []string{"uuid", "", "uuid"})
	assert.Equal(t, "$1::uuid, $2, $3::uuid", gotPg)
}
