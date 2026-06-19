package version

import (
	"runtime"
	"runtime/debug"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// PrometheusBuildInfoLabels returns (version, revision, go_version) for the
// taskapi_build_info gauge. version matches String(); revision is the first 12
// characters of vcs.revision when present, else "unknown"; go_version is runtime.Version().
func PrometheusBuildInfoLabels() (ver, rev, gover string) {
	gover = runtime.Version()
	ver = String()
	rev = "unknown"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ver, rev, gover
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			r := s.Value
			if len(r) > 12 {
				r = r[:12]
			}
			rev = r
			break
		}
	}
	return ver, rev, gover
}
