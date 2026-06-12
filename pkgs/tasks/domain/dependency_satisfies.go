package domain

import "strings"

// DependencySatisfies names the predecessor condition a dependency edge
// requires before the dependent task may dequeue.
type DependencySatisfies string

const (
	// DependencySatisfiesDone requires predecessor status=done (default).
	DependencySatisfiesDone DependencySatisfies = "done"
	// DependencySatisfiesCriteriaComplete requires the predecessor's
	// inherited checklist to be fully verified (criteria_satisfied_at set).
	DependencySatisfiesCriteriaComplete DependencySatisfies = "criteria_complete"
)

// ValidDependencySatisfies reports whether s is a known edge predicate.
func ValidDependencySatisfies(s DependencySatisfies) bool {
	switch s {
	case DependencySatisfiesDone, DependencySatisfiesCriteriaComplete, "":
		return true
	default:
		return false
	}
}

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
