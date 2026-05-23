// Package all imports every runner adapter so their init() functions
// register with the global registry. Binaries that need runner
// support import this package for the side effect:
//
//	import _ "github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry/all"
package all

import (
	_ "github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
)
