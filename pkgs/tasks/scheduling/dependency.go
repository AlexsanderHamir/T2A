package scheduling

import (
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// EdgeSatisfied reports whether predecessor meets the edge predicate.
func EdgeSatisfied(predecessor *domain.Task, satisfies domain.DependencySatisfies) bool {
	if predecessor == nil {
		return false
	}
	_ = satisfies
	return predecessor.Status == domain.StatusDone
}
