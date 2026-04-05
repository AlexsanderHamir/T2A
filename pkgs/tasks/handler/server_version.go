package handler

import "runtime/debug"

// ServerVersion returns a short identifier for this binary for health JSON and support
// correlation. It prefers a non-devel module version, else a short VCS revision, else
// "devel" or "unknown". Safe to expose on /health (no secrets).
func ServerVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
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
	if info.Main.Version == "(devel)" {
		return "devel"
	}
	return "unknown"
}
