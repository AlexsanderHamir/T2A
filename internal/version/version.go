// Package version reports a short build identifier from runtime module metadata.
package version

import "runtime/debug"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// String returns a short identifier for this binary (release module version, short
// vcs.revision, "devel", or "unknown"). Safe to log and expose on HTTP health JSON
// (no secrets).
func String() string {
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
