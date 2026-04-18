package coldindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSetextUnderline(t *testing.T) {
	tests := []struct {
		name          string
		trimmed       string
		expectedLevel int
		expectedOK    bool
	}{
		{"level 1 equals", "===", 1, true},
		{"level 2 dashes", "---", 2, true},
		{"too short", "==", 0, false},
		{"mixed", "=-=", 0, false},
		{"with spaces", "= = =", 1, true},
		{"not underline", "hello", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, ok := parseSetextUnderline(tt.trimmed)
			assert.Equal(t, tt.expectedLevel, level)
			assert.Equal(t, tt.expectedOK, ok)
		})
	}
}

func TestExtractSingleOrderedPrefix(t *testing.T) {
	tests := []struct {
		name         string
		trim         string
		expectedN    int
		expectedRest string
		expectedOK   bool
	}{
		{"simple", "1. Hello", 1, "Hello", true},
		{"multi digit", "12. World", 12, "World", true},
		{"chinese", "1. 概述", 1, "概述", true},
		{"no number", "Hello", 0, "", false},
		{"no dot", "1 Hello", 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, rest, ok := extractSingleOrderedPrefix(tt.trim)
			assert.Equal(t, tt.expectedN, n)
			assert.Equal(t, tt.expectedRest, rest)
			assert.Equal(t, tt.expectedOK, ok)
		})
	}
}

func TestShortOrderedListTriple(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		start    int
		expected bool
	}{
		{
			name:     "triple list",
			lines:    []string{"1. a", "2. b", "3. c"},
			start:    0,
			expected: true,
		},
		{
			name:     "not sequential",
			lines:    []string{"1. a", "3. b", "4. c"},
			start:    0,
			expected: false,
		},
		{
			name:     "too long",
			lines:    []string{"1. this is a very long text", "2. b", "3. c"},
			start:    0,
			expected: false,
		},
		{
			name:     "multi dot",
			lines:    []string{"1.1. a", "1.2. b", "1.3. c"},
			start:    0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, shortOrderedListTriple(tt.lines, tt.start))
		})
	}
}

func TestPartOfShortOrderedListTriple(t *testing.T) {
	lines := []string{"1. a", "2. b", "3. c"}

	// First line
	assert.True(t, partOfShortOrderedListTriple(lines, 0))

	// Middle line
	assert.True(t, partOfShortOrderedListTriple(lines, 1))

	// Last line
	assert.True(t, partOfShortOrderedListTriple(lines, 2))

	// Too short
	shortLines := []string{"1. a", "2. b"}
	assert.False(t, partOfShortOrderedListTriple(shortLines, 0))
}

func TestSanitizePathPart(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello/world", "hello-world"},
		{"hello\\world", "hello-world"},
		{"  spaced  ", "spaced"},
		{"", "body"},
		{string(make([]byte, 200)), string(make([]byte, 120))},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 20)], func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizePathPart(tt.input))
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestSplitFirstLine(t *testing.T) {
	tests := []struct {
		rest          string
		expectedLine  string
		expectedAfter string
	}{
		{"line1\nline2", "line1", "line2"},
		{"single", "single", ""},
		{"", "", ""},
		{"line1\nline2\nline3", "line1", "line2\nline3"},
	}

	for _, tt := range tests {
		t.Run(tt.rest[:min(len(tt.rest), 20)], func(t *testing.T) {
			line, after := splitFirstLine(tt.rest)
			assert.Equal(t, tt.expectedLine, line)
			assert.Equal(t, tt.expectedAfter, after)
		})
	}
}

func TestNormalizeEOL(t *testing.T) {
	assert.Equal(t, "a\nb\nc", normalizeEOL("a\r\nb\rc"))
	assert.Equal(t, "a\nb", normalizeEOL("a\nb"))
}
