package domain

// PhaseInterruptReason is written into task_cycle_phases.summary when the
// agent worker finalizes a running phase after process interruption. Resume
// logic treats this as permission to re-enter the same phase enum.
const PhaseInterruptReason = "process_restart"

// ValidPhaseTransition reports whether a cycle may move from prev to next within
// the same cycle. Empty prev means "no prior phase" (cycle just started); empty
// next is rejected (callers always know what they want to enter).
//
// The pipeline is `execute → verify`, with one corrective edge `verify →
// execute` for retries after a failing verification. A second corrective edge
// `verify → verify` is allowed via ValidVerifyOnlyRetryTransition (ADR-0028)
// when execute is skipped on infra-only verify retries. Verify is terminal
// within the cycle: the cycle itself then moves to a terminal status via
// TerminateCycle. Re-entering execute without an intervening verify is
// illegal; store-side PhaseSeq distinguishes the per-attempt rows of repeated
// phases.
//
// Skip-listed in cmd/funclogmeasure/analyze.go: pure predicate with no I/O;
// every caller (store.StartPhase, store.CompletePhase) logs the transition
// decision with rich context, so logging here would emit a redundant trace
// line per phase mutation.
func ValidPhaseTransition(prev, next Phase) bool {
	if next == "" {
		return false
	}
	if prev == "" {
		return next == PhaseExecute
	}
	switch prev {
	case PhaseExecute:
		return next == PhaseVerify
	case PhaseVerify:
		return next == PhaseExecute
	default:
		return false
	}
}

// ValidInterruptResumeTransition reports whether the cycle may open another
// row with the same phase enum immediately after process-interrupt finalization.
// last must be the highest-seq phase row: terminal failed with summary
// PhaseInterruptReason and the same phase kind as next.
func ValidInterruptResumeTransition(last *TaskCyclePhase, next Phase) bool {
	if last == nil || next == "" {
		return false
	}
	if last.Phase != next {
		return false
	}
	if !TerminalPhaseStatus(last.Status) || last.Status != PhaseStatusFailed {
		return false
	}
	if last.Summary == nil || *last.Summary != PhaseInterruptReason {
		return false
	}
	return true
}

// ValidVerifyOnlyRetryTransition reports whether the cycle may open another
// verify phase immediately after a terminal failed verify without an
// intervening execute phase (ADR-0028 in-cycle verify-only retry). last must
// be the highest-seq phase row: terminal failed verify.
func ValidVerifyOnlyRetryTransition(last *TaskCyclePhase, next Phase) bool {
	if last == nil || next != PhaseVerify {
		return false
	}
	if last.Phase != PhaseVerify {
		return false
	}
	return TerminalPhaseStatus(last.Status) && last.Status == PhaseStatusFailed
}

// TerminalCycleStatus reports whether s is a final, immutable cycle status.
// Callers must not mutate cycles whose status is terminal; new attempts get a
// new TaskCycle row with a higher AttemptSeq. Skip-listed in
// cmd/funclogmeasure/analyze.go: pure predicate (see ValidPhaseTransition).
func TerminalCycleStatus(s CycleStatus) bool {
	switch s {
	case CycleStatusSucceeded, CycleStatusFailed, CycleStatusAborted:
		return true
	default:
		return false
	}
}

// TerminalPhaseStatus reports whether s is a final, immutable phase status.
// Once a phase reaches a terminal status its row is read-only; corrective work
// inside the same cycle creates a new TaskCyclePhase row with a higher
// PhaseSeq. Skip-listed for the same reason as TerminalCycleStatus above.
func TerminalPhaseStatus(s PhaseStatus) bool {
	switch s {
	case PhaseStatusSucceeded, PhaseStatusFailed, PhaseStatusSkipped:
		return true
	default:
		return false
	}
}
