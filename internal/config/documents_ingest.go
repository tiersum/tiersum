package config

import (
	"strings"

	"github.com/spf13/viper"
)

// DocumentMaxBodyBytes is the maximum ingest body size (UTF-8 byte length of content).
func DocumentMaxBodyBytes() int64 {
	return ParseHumanBytes(viper.GetString("documents.max_size"), 50<<20)
}

// DocumentFormatAllowed reports whether the format string is allowed for ingest.
// When documents.supported_formats is empty, any non-empty format is allowed.
func DocumentFormatAllowed(format string) bool {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return false
	}
	allowed := viper.GetStringSlice("documents.supported_formats")
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if strings.ToLower(strings.TrimSpace(a)) == format {
			return true
		}
	}
	return false
}

// DocumentChunkingMaxChars returns (enabled, maxRunes). When enabled and maxRunes > 0,
// ingest rejects content longer than maxRunes (Unicode code points, counted with utf8.RuneCountInString).
func DocumentChunkingMaxChars() (enabled bool, maxRunes int) {
	if !viper.GetBool("documents.chunking.enabled") {
		return false, 0
	}
	n := viper.GetInt("documents.chunking.max_chunk_size")
	if n <= 0 {
		return true, 10000
	}
	return true, n
}
