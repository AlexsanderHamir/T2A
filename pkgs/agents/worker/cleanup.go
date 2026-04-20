package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// cleanup.go owns every non-happy-path closeout for a single task:
//
//   - handleShutdownAfterRun  parent ctx cancelled mid-run (edge #5)
//   - recoverFromPanic        deferred panic safety net   (edge #4)
//   - bestEffortFailTask      StartCycle failed, task is "running"
//   - bestEffortTerminate     StartPhase / diagnose write tripped
//
// Each path runs on a non-cancelled background context bounded by
// Options.ShutdownAbortTimeout so even a dead parent ctx leaves the
// audit trail honest. The startup orphan sweep
// (cmd/taskapi.startAgentWorkerIfEnabled) is the safety net if even
// these best-effort writes fail to land.

// handleShutdownAfterRun closes out the in-flight cycle on a
// non-cancelled background context so the audit row lands even after
// the parent ctx is dead. The startup sweep in Stage 4 is the safety
// net if even this best-effort write trips its deadline.
func (w *Worker) handleShutdownAfterRun(state *processState, taskID string) {
	slog.Info("agent worker shutdown mid-run, finalizing cycle as aborted",
		"cmd", workerLogCmd, "operation", "agent.worker.Worker.handleShutdownAfterRun",
		"task_id", taskID, "cycle_id", state.cycleID)
	bg, cancel := context.WithTimeout(context.Background(), w.options.ShutdownAbortTimeout)
	defer cancel()
	if state.runningPhaseSeq > 0 {
		summary := ShutdownReason
		if _, err := w.store.CompletePhase(bg, store.CompletePhaseInput{
			CycleID:  state.cycleID,
			PhaseSeq: state.runningPhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			slog.Warn("agent worker shutdown CompletePhase failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.handleShutdownAfterRun.complete_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			w.publish(taskID, state.cycleID)
		}
		state.runningPhase = ""
		state.runningPhaseSeq = 0
	}
	if state.cycleStarted {
		if _, err := w.store.TerminateCycle(bg, state.cycleID, domain.CycleStatusAborted, ShutdownReason, domain.ActorAgent); err != nil {
			slog.Warn("agent worker shutdown TerminateCycle failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.handleShutdownAfterRun.terminate_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			w.publish(taskID, state.cycleID)
			w.recordRun(string(domain.CycleStatusAborted), w.runner.Name(), state.effectiveModel, state.startedAt)
		}
		state.cycleStarted = false
	}
	failed := domain.StatusFailed
	if _, err := w.store.Update(bg, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker shutdown task transition failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.handleShutdownAfterRun.task_err",
				"task_id", taskID, "err", err)
		}
	}
}

// recoverFromPanic is the deferred safety net for any panic inside
// processOne (typically inside runner.Run). It mirrors the shutdown
// branch's "background context + bounded deadline" pattern so even a
// catastrophic failure leaves the audit trail honest. The Run loop
// keeps going on the next Receive.
func (w *Worker) recoverFromPanic(state *processState, task domain.Task) {
	r := recover()
	if r == nil {
		slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.recoverFromPanic.no_panic")
		return
	}
	slog.Error("agent worker panic", "cmd", workerLogCmd,
		"operation", "agent.worker.Worker.recoverFromPanic", "task_id", task.ID,
		"cycle_id", state.cycleID, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
	bg, cancel := context.WithTimeout(context.Background(), w.options.ShutdownAbortTimeout)
	defer cancel()
	if state.runningPhaseSeq > 0 {
		summary := PanicReason
		if _, err := w.store.CompletePhase(bg, store.CompletePhaseInput{
			CycleID:  state.cycleID,
			PhaseSeq: state.runningPhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			slog.Warn("agent worker panic CompletePhase failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.recoverFromPanic.complete_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			w.publish(task.ID, state.cycleID)
		}
		state.runningPhase = ""
		state.runningPhaseSeq = 0
	}
	if state.cycleStarted {
		if _, err := w.store.TerminateCycle(bg, state.cycleID, domain.CycleStatusFailed, PanicReason, domain.ActorAgent); err != nil {
			slog.Warn("agent worker panic TerminateCycle failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.recoverFromPanic.terminate_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			w.publish(task.ID, state.cycleID)
			w.recordRun(string(domain.CycleStatusFailed), w.runner.Name(), state.effectiveModel, state.startedAt)
		}
		state.cycleStarted = false
	}
	failed := domain.StatusFailed
	if _, err := w.store.Update(bg, task.ID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker panic task transition failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.recoverFromPanic.task_err",
				"task_id", task.ID, "err", err)
		}
	}
}

// bestEffortFailTask is the cleanup path used when StartCycle itself
// failed (so there is no cycle row to terminate but the task is now
// `running` and would otherwise be re-enqueued forever by the
// reconcile loop). See docs/AGENT-WORKER.md "Lifecycle of one task".
func (w *Worker) bestEffortFailTask(ctx context.Context, taskID string) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.bestEffortFailTask",
		"task_id", taskID)
	failed := domain.StatusFailed
	if _, err := w.store.Update(ctx, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker bestEffortFailTask failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.bestEffortFailTask.err",
				"task_id", taskID, "err", err)
		}
	}
}

// bestEffortTerminate closes a cycle that was opened but whose phase
// pipeline tripped before runner.Run; used when StartPhase or the
// CompletePhase for the skipped-diagnose row failed. Best-effort: store
// errors are logged and swallowed, the startup sweep is the safety net.
func (w *Worker) bestEffortTerminate(ctx context.Context, state *processState, taskID string, status domain.CycleStatus, reason string) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.bestEffortTerminate",
		"cycle_id", state.cycleID, "status", string(status), "reason", reason)
	if state.runningPhaseSeq > 0 {
		summary := reason
		if _, err := w.store.CompletePhase(ctx, store.CompletePhaseInput{
			CycleID:  state.cycleID,
			PhaseSeq: state.runningPhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				slog.Warn("agent worker bestEffortTerminate CompletePhase failed",
					"cmd", workerLogCmd,
					"operation", "agent.worker.Worker.bestEffortTerminate.complete_err",
					"cycle_id", state.cycleID, "err", err)
			}
		} else {
			w.publish(taskID, state.cycleID)
		}
		state.runningPhase = ""
		state.runningPhaseSeq = 0
	}
	if state.cycleStarted {
		if _, err := w.store.TerminateCycle(ctx, state.cycleID, status, reason, domain.ActorAgent); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				slog.Warn("agent worker bestEffortTerminate TerminateCycle failed",
					"cmd", workerLogCmd,
					"operation", "agent.worker.Worker.bestEffortTerminate.terminate_err",
					"cycle_id", state.cycleID, "err", err)
			}
		} else {
			w.publish(taskID, state.cycleID)
			w.recordRun(string(status), w.runner.Name(), state.effectiveModel, state.startedAt)
		}
		state.cycleStarted = false
	}
	w.bestEffortFailTask(ctx, taskID)
}
