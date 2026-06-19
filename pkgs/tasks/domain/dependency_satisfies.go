package domain

import "strings"

// DependencySatisfies names the predecessor condition a dependency edge
// requires before the dependent task may dequeue.
type DependencySatisfies string

const (
	// DependencySatisfiesDone requires predecessor status=done (default).
	DependencySatisfiesDone DependencySatisfies = "done"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidDependencySatisfies reports whether s is a known edge predicate.
func ValidDependencySatisfies(s DependencySatisfies) bool {
	switch s {
	case DependencySatisfiesDone, "":
		return true
	default:
		return false
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NormalizeDependencySatisfies returns the canonical predicate, defaulting
// empty to done.
func NormalizeDependencySatisfies(s DependencySatisfies) DependencySatisfies {
	if strings.TrimSpace(string(s)) == "" {
		return DependencySatisfiesDone
	}
	return DependencySatisfies(s)
}

// DependencyEdge is a hydrated depends_on row for API and store layers.
type DependencyEdge struct {
	TaskID    string              `json:"task_id"`
	Satisfies DependencySatisfies `json:"satisfies,omitempty"`
}
