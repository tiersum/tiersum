package api

import "runtime/debug"

// moduleVersion returns the main module version from build info (e.g. v1.2.3 or "(devel)").
func moduleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
