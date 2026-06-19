package handler

import "github.com/AlexsanderHamir/T2A/internal/version"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ServerVersion returns the same build identifier as internal/version.String (module
// release tag, short VCS revision, devel, or unknown). Safe to expose on /health.
func ServerVersion() string {
	return version.String()
}
