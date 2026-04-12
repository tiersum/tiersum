package config

import (
	"strconv"
	"strings"
)

// ParseHumanBytes parses sizes like "10MB", "50MiB", "1048576". Suffixes are case-insensitive.
// MB/MiB use 1024*1024; GB uses 1024^3. Plain digits are interpreted as bytes.
// Returns defaultBytes when s is empty or cannot be parsed.
func ParseHumanBytes(s string, defaultBytes int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultBytes
	}
	u := strings.ToUpper(strings.ReplaceAll(s, " ", ""))

	var mult int64 = 1
	switch {
	case strings.HasSuffix(u, "GIB"):
		mult = 1 << 30
		u = strings.TrimSuffix(u, "GIB")
	case strings.HasSuffix(u, "GB"):
		mult = 1024 * 1024 * 1024
		u = strings.TrimSuffix(u, "GB")
	case strings.HasSuffix(u, "MIB"):
		mult = 1 << 20
		u = strings.TrimSuffix(u, "MIB")
	case strings.HasSuffix(u, "MB"):
		mult = 1024 * 1024
		u = strings.TrimSuffix(u, "MB")
	case strings.HasSuffix(u, "KIB"):
		mult = 1 << 10
		u = strings.TrimSuffix(u, "KIB")
	case strings.HasSuffix(u, "KB"):
		mult = 1024
		u = strings.TrimSuffix(u, "KB")
	}

	n, err := strconv.ParseFloat(u, 64)
	if err != nil || n <= 0 {
		return defaultBytes
	}
	v := int64(n * float64(mult))
	if v <= 0 {
		return defaultBytes
	}
	return v
}
