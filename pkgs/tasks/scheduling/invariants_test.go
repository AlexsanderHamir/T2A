package scheduling_test

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/scheduling"
)

// Documents ADR-0023 invariants enforced by unit + parity tests in this package.
func TestInvariants_documentedCoverage(t *testing.T) {
	t.Parallel()
	// I1 Admission ⊆ readiness — worker/processOne + store ReadyForAgentPickup (integration).
	// I2 Reconcile ⊆ SQL dequeuable — ready.ListQueueCandidates + agentreconcile tests.
	// I3 Go ≡ SQL — parity_test.go TestParity_GoReadinessMatchesListQueueCandidates.
	// I4 Pickup enqueue gate — pickup_test.go + decide_notify_test future pickup case.
	// I5 Dependent wake ⊆ readiness — facade notifyUnblockedDependents uses full readiness.
	// I6 Persist beats notify — unchanged store transaction boundaries.
	// I7 Enqueue ≠ admission — DecideNotifyAfterReadyTransition pickup-only notify path.
	_ = scheduling.FailedPredicateNone
}
