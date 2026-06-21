package scheduling

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// EdgeSatisfied reports whether predecessor meets the edge predicate.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func EdgeSatisfied(predecessor *domain.Task, satisfies domain.DependencySatisfies) bool {
	if predecessor == nil {
		return false
	}
	_ = satisfies
	return predecessor.Status == domain.StatusDone
}
