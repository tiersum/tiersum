package api

import (
	"runtime/debug"
	"strings"
)

// moduleVersion returns a best-effort build label: Go module version, VCS revision, or "dev".
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := strings.TrimSpace(info.Main.Version); v != "" {
		return v
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			rev := s.Value
			if len(rev) > 12 {
				return rev[:12]
			}
			return rev
		}
	}
	return "dev"
}
