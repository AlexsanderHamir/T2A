package scheduling

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestEdgeSatisfied_doneOnly(t *testing.T) {
	t.Parallel()
	done := &domain.Task{Status: domain.StatusDone}
	ready := &domain.Task{Status: domain.StatusReady}
	if !EdgeSatisfied(done, domain.DependencySatisfiesDone) {
		t.Fatal("done predecessor should satisfy")
	}
	if EdgeSatisfied(ready, domain.DependencySatisfiesDone) {
		t.Fatal("ready predecessor should not satisfy")
	}
}
