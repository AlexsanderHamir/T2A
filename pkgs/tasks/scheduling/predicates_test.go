package scheduling

import (
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestEvaluateWorkerReadiness_predicateOrder(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	gateHeld := &domain.TaskGate{Kind: domain.GateKindManualApproval, Status: domain.GateStatusActive}

	cases := []struct {
		name      string
		task      *domain.Task
		depsMet   bool
		wantReady bool
		wantPred  FailedPredicate
	}{
		{"nil task", nil, true, false, FailedPredicateStatus},
		{"not ready", &domain.Task{Status: domain.StatusBlocked}, true, false, FailedPredicateStatus},
		{"future pickup", &domain.Task{Status: domain.StatusReady, PickupNotBefore: &future}, true, false, FailedPredicatePickup},
		{"held gate", &domain.Task{Status: domain.StatusReady, Gate: gateHeld}, true, false, FailedPredicateGate},
		{"open dependency", &domain.Task{Status: domain.StatusReady}, false, false, FailedPredicateDependencies},
		{"all clear", &domain.Task{Status: domain.StatusReady, PickupNotBefore: &past}, true, true, FailedPredicateNone},
		{"nil pickup", &domain.Task{Status: domain.StatusReady}, true, true, FailedPredicateNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvaluateWorkerReadiness(c.task, now, c.depsMet)
			if got.Ready != c.wantReady || got.FailedPredicate != c.wantPred {
				t.Fatalf("got %+v want ready=%v pred=%q", got, c.wantReady, c.wantPred)
			}
		})
	}
}
