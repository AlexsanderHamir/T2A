package handler

import "github.com/AlexsanderHamir/Hamix/internal/version"

// ServerVersion returns the same build identifier as internal/version.String (module
// release tag, short VCS revision, devel, or unknown). Safe to expose on /health.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ServerVersion() string {
	return version.String()
}
