package domain

import (
	"context"
	"log/slog"
)

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
func ValidPhaseTransition(prev, next Phase) bool {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.ValidPhaseTransition")
	}
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
// new TaskCycle row with a higher AttemptSeq.
func TerminalCycleStatus(s CycleStatus) bool {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.TerminalCycleStatus")
	}
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
// PhaseSeq.
func TerminalPhaseStatus(s PhaseStatus) bool {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.TerminalPhaseStatus")
	}
	switch s {
	case PhaseStatusSucceeded, PhaseStatusFailed, PhaseStatusSkipped:
		return true
	default:
		return false
	}
}
