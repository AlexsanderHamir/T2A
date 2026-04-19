package domain

// ValidPhaseTransition reports whether a cycle may move from prev to next within
// the same cycle. Empty prev means "no prior phase" (cycle just started); empty
// next is rejected (callers always know what they want to enter).
//
// The allowed transitions follow the diagnose -> execute -> verify -> persist
// loop from moat.md, plus one corrective edge: Verify may loop back to Execute
// when verification fails and a corrective Execute is required. Persist is
// terminal within the cycle (the cycle itself then moves to a terminal status).
//
// Re-entering the same phase enum is also allowed (for example a second
// Execute after a failing Verify); store-side PhaseSeq distinguishes the two
// rows. The state machine only constrains the transition graph, not how many
// times a node may be visited.
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
		return next == PhaseDiagnose
	}
	switch prev {
	case PhaseDiagnose:
		return next == PhaseExecute
	case PhaseExecute:
		return next == PhaseVerify
	case PhaseVerify:
		return next == PhasePersist || next == PhaseExecute
	case PhasePersist:
		return false
	default:
		return false
	}
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
