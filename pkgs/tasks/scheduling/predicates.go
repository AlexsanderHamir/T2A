package scheduling

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// EvaluateWorkerReadiness applies the four worker predicates in fixed order.
// dependenciesMet must reflect store-loaded edge satisfaction when predicate 4 applies.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func EvaluateWorkerReadiness(task *domain.Task, now time.Time, dependenciesMet bool) ReadinessResult {
	if task == nil || task.Status != domain.StatusReady {
		return ReadinessResult{Ready: false, FailedPredicate: FailedPredicateStatus}
	}
	if task.PickupNotBefore != nil && task.PickupNotBefore.After(now) {
		return ReadinessResult{Ready: false, FailedPredicate: FailedPredicatePickup}
	}
	if task.Gate != nil && task.Gate.GateBlocksWorker() {
		return ReadinessResult{Ready: false, FailedPredicate: FailedPredicateGate}
	}
	if !dependenciesMet {
		return ReadinessResult{Ready: false, FailedPredicate: FailedPredicateDependencies}
	}
	return ReadinessResult{Ready: true, FailedPredicate: FailedPredicateNone}
}
