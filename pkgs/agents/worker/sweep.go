package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// SweepReason is the cycle/phase termination reason recorded by the
// startup orphan sweep. Pinned so audit consumers (UI, alerts) can
// distinguish process-restart cleanup from in-band failures.
const SweepReason = "process_restart"

// SweepResult is the structured outcome of one
// SweepOrphanRunningCycles call. The counts are best-effort aggregates
// over the rows actually mutated, not the rows looked at: an orphan
// row whose terminate write fails (e.g. ErrNotFound from a concurrent
// delete) is logged and silently skipped, not double-counted.
type SweepResult struct {
	// PhasesFailed is the number of task_cycle_phases rows the sweep
	// successfully flipped from running to failed.
	PhasesFailed int
	// CyclesAborted is the number of task_cycles rows the sweep
	// successfully flipped from running to aborted.
	CyclesAborted int
	// TasksFailed is the number of tasks the sweep successfully
	// transitioned from running to failed (one per aborted cycle when
	// the underlying task is still running).
	TasksFailed int
}

// SweepOrphanRunningCycles closes any cycle/phase rows left in
// status='running' by a previous process. It is the safety net for
// edge case #5 (graceful shutdown mid-run): if the worker's deferred
// best-effort writes ever fail to land — power loss, OS kill, deadline
// trip — the next process restart calls this once before the worker
// loop begins so the audit trail and task lifecycle stay honest.
//
// Behaviour:
//   - For every running phase row (whether under a still-running cycle
//     or one that already terminated): CompletePhase(failed,
//     SweepReason). Phase-first order avoids the "cycle has running
//     phase" guard inside Terminate.
//   - For every running cycle row: TerminateCycle(aborted, SweepReason).
//   - For each aborted cycle whose owning task is still in
//     StatusRunning: Update(task, failed). Tasks already in any other
//     status are left alone — the sweep is restart-safe and never
//     overwrites manual or REST-driven state.
//
// Each store call uses ctx; on context cancellation the function
// returns whatever it has accumulated so far plus the underlying
// error. Per-row store errors are logged and skipped so one bad row
// cannot block the whole sweep.
//
// Idempotent: re-running on a clean DB is a no-op.
func SweepOrphanRunningCycles(ctx context.Context, st *store.Store) (SweepResult, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.SweepOrphanRunningCycles")
	var res SweepResult
	if st == nil {
		return res, errors.New("agent worker sweep: nil store")
	}

	phases, err := st.ListRunningCyclePhases(ctx)
	if err != nil {
		return res, err
	}
	for _, p := range phases {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		summary := SweepReason
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
			slog.Log(ctx, level, "agent worker sweep CompletePhase failed",
				"cmd", workerLogCmd, "operation", "agent.worker.SweepOrphanRunningCycles.complete_err",
				"cycle_id", p.CycleID, "phase_seq", p.PhaseSeq, "err", err)
			continue
		}
		res.PhasesFailed++
	}

	cycles, err := st.ListRunningCycles(ctx)
	if err != nil {
		return res, err
	}
	for _, c := range cycles {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if _, err := st.TerminateCycle(ctx, c.ID, domain.CycleStatusAborted, SweepReason, domain.ActorAgent); err != nil {
			level := slog.LevelWarn
			if errors.Is(err, domain.ErrNotFound) {
				level = slog.LevelInfo
			}
			slog.Log(ctx, level, "agent worker sweep TerminateCycle failed",
				"cmd", workerLogCmd, "operation", "agent.worker.SweepOrphanRunningCycles.terminate_err",
				"cycle_id", c.ID, "task_id", c.TaskID, "err", err)
			continue
		}
		res.CyclesAborted++

		if walked := walkTaskToFailedIfRunning(ctx, st, c.TaskID); walked {
			res.TasksFailed++
		}
	}

	slog.Info("agent worker startup sweep complete", "cmd", workerLogCmd,
		"operation", "agent.worker.SweepOrphanRunningCycles.summary",
		"phases_failed", res.PhasesFailed, "cycles_aborted", res.CyclesAborted,
		"tasks_failed", res.TasksFailed)
	return res, nil
}

// walkTaskToFailedIfRunning reads the task and only transitions it to
// failed when its current status is StatusRunning. Tasks the sweep
// finds in any other status are left alone — they were either already
// closed by an in-band path or driven there manually via the REST API,
// and the sweep must not double-write a status_changed event in either
// case.
func walkTaskToFailedIfRunning(ctx context.Context, st *store.Store, taskID string) bool {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.walkTaskToFailedIfRunning",
		"task_id", taskID)
	task, err := st.Get(ctx, taskID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker sweep task lookup failed", "cmd", workerLogCmd,
				"operation", "agent.worker.walkTaskToFailedIfRunning.get_err",
				"task_id", taskID, "err", err)
		}
		return false
	}
	if task.Status != domain.StatusRunning {
		return false
	}
	failed := domain.StatusFailed
	if _, err := st.Update(ctx, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent worker sweep task transition failed",
			"cmd", workerLogCmd, "operation", "agent.worker.walkTaskToFailedIfRunning.update_err",
			"task_id", taskID, "err", err)
		return false
	}
	return true
}
