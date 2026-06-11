package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// InterruptPhaseReason is the phase summary recorded when a running phase
// is closed by startup finalization after process interruption. Pinned so
// audit consumers and resume logic can distinguish restart cleanup from
// in-band failures.
const InterruptPhaseReason = domain.PhaseInterruptReason

// SweepReason is kept as an alias for callers and tests that referenced
// the historical orphan-sweep constant.
const SweepReason = InterruptPhaseReason

// FinalizeResult is the structured outcome of one FinalizeInterruptedPhases
// call. The counts are best-effort aggregates over rows actually mutated.
type FinalizeResult struct {
	// PhasesFinalized is the number of task_cycle_phases rows successfully
	// flipped from running to failed with InterruptPhaseReason.
	PhasesFinalized int
}

// SweepResult is retained for compatibility with existing log fields and
// tests during the transition away from fail-all orphan sweep.
type SweepResult struct {
	PhasesFailed  int
	CyclesAborted int
	TasksFailed   int
}

// FinalizeInterruptedPhases closes any phase rows left in status='running'
// by a previous process without aborting cycles or failing tasks. Running
// cycles stay running so Harness.Resume can continue the same attempt.
//
// Idempotent: re-running on a clean DB is a no-op.
func FinalizeInterruptedPhases(ctx context.Context, st *store.Store) (FinalizeResult, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.FinalizeInterruptedPhases")
	var res FinalizeResult
	if st == nil {
		return res, errors.New("agent worker finalize: nil store")
	}

	phases, err := st.ListRunningCyclePhases(ctx)
	if err != nil {
		return res, err
	}
	for _, p := range phases {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		summary := InterruptPhaseReason
		if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
			CycleID:  p.CycleID,
			PhaseSeq: p.PhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			level := slog.LevelWarn
			if errors.Is(err, domain.ErrNotFound) {
				level = slog.LevelInfo
			}
			slog.Log(ctx, level, "agent worker finalize CompletePhase failed",
				"cmd", workerLogCmd, "operation", "agent.worker.FinalizeInterruptedPhases.complete_err",
				"cycle_id", p.CycleID, "phase_seq", p.PhaseSeq, "err", err)
			continue
		}
		res.PhasesFinalized++
	}

	slog.Info("agent worker startup finalize complete", "cmd", workerLogCmd,
		"operation", "agent.worker.FinalizeInterruptedPhases.summary",
		"phases_finalized", res.PhasesFinalized)
	return res, nil
}

// SweepOrphanRunningCycles is deprecated: it now delegates to
// FinalizeInterruptedPhases only. Cycles and tasks are no longer aborted
// or failed on startup.
func SweepOrphanRunningCycles(ctx context.Context, st *store.Store) (SweepResult, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.SweepOrphanRunningCycles")
	fr, err := FinalizeInterruptedPhases(ctx, st)
	if err != nil {
		return SweepResult{}, err
	}
	return SweepResult{PhasesFailed: fr.PhasesFinalized}, nil
}
