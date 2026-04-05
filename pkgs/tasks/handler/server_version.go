package handler

import "github.com/AlexsanderHamir/T2A/internal/version"

// ServerVersion returns the same build identifier as internal/version.String (module
// release tag, short VCS revision, devel, or unknown). Safe to expose on /health.
func ServerVersion() string {
	return version.String()
}
