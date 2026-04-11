package telemetry

import (
	"strings"

	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("telemetry.enabled", false)
	viper.SetDefault("telemetry.persist_to_db", true)
	viper.SetDefault("telemetry.sample_ratio", 0.1)
	viper.SetDefault("telemetry.service_name", "tiersum")
	viper.SetDefault("telemetry.force_sample_query_param", "debug_trace")
	viper.SetDefault("telemetry.force_sample_header", "X-TierSum-Debug-Trace")
}

// GlobalHTTPTracingConfigured reports whether HTTP middleware should attach OpenTelemetry spans.
func GlobalHTTPTracingConfigured() bool {
	return viper.GetBool("telemetry.enabled") && viper.GetBool("telemetry.persist_to_db")
}

// TruthyQuery reports whether a query parameter is set to a truthy value (1, true, yes).
func TruthyQuery(c interface{ Query(string) string }, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	return isTruthy(strings.TrimSpace(c.Query(name)))
}

// TruthyHeader reports whether a header is set to a truthy value.
func TruthyHeader(c interface{ GetHeader(string) string }, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	return isTruthy(strings.TrimSpace(strings.ToLower(c.GetHeader(name))))
}

func isTruthy(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
