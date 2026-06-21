package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// cleanup.go owns every non-happy-path closeout for a single task:
//
//   - handleShutdownAfterRun  parent ctx cancelled mid-run (edge #5)
//   - recoverFromPanic        deferred panic safety net   (edge #4)
//   - bestEffortFailTask      StartCycle failed, task is "running"
//   - bestEffortTerminate     StartPhase / phase write tripped
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
func (h *Harness) handleShutdownAfterRun(state *processState, taskID string) {
	slog.Info("agent harness shutdown mid-run, finalizing cycle as aborted",
		"cmd", harnessLogCmd, "operation", "agent.harness.Harness.handleShutdownAfterRun",
		"task_id", taskID, "cycle_id", state.cycleID)
	bg, cancel := context.WithTimeout(context.Background(), h.opts.ShutdownAbortTimeout)
	defer cancel()
	if state.runningPhaseSeq > 0 {
		summary := ShutdownReason
		if _, err := h.store.CompletePhase(bg, store.CompletePhaseInput{
			CycleID:  state.cycleID,
			PhaseSeq: state.runningPhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			slog.Warn("agent harness shutdown CompletePhase failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.handleShutdownAfterRun.complete_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			h.publish(taskID, state.cycleID)
		}
		state.runningPhase = ""
		state.runningPhaseSeq = 0
	}
	if state.cycleStarted {
		if _, err := h.store.TerminateCycle(bg, state.cycleID, domain.CycleStatusAborted, ShutdownReason, domain.ActorAgent); err != nil {
			slog.Warn("agent harness shutdown TerminateCycle failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.handleShutdownAfterRun.terminate_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			h.publish(taskID, state.cycleID)
			h.recordRun(string(domain.CycleStatusAborted), h.runner.Name(), state.effectiveModel, state.startedAt)
		}
		state.cycleStarted = false
	}
	failed := domain.StatusFailed
	if _, err := h.store.Update(bg, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent harness shutdown task transition failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.handleShutdownAfterRun.task_err",
				"task_id", taskID, "err", err)
		}
	}
	h.cleanupCycleReports(state.cycleID, "shutdown")
}

// cleanupCycleReports is the shared GC for the worker's scratch dir.
// Every cycle exit path (happy terminate, panic, shutdown, best-effort)
// calls it so disk growth stays bounded regardless of how the cycle
// ends. Idempotent and best-effort: a missing dir is normal, a real
// error is logged but never propagated.
func (h *Harness) cleanupCycleReports(cycleID, reason string) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.cleanupCycleReports",
		"cycle_id", cycleID, "reason", reason)
	if cycleID == "" {
		return
	}
	if err := reports.CleanupReportDir(h.opts.ReportDir, cycleID); err != nil {
		slog.Warn("agent harness cleanupCycleReports failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.cleanupCycleReports.err",
			"cycle_id", cycleID, "report_dir", h.opts.ReportDir, "reason", reason, "err", err)
	}
}

// recoverFromPanic is the deferred safety net for any panic inside
// processOne (typically inside runner.Run). It mirrors the shutdown
// branch's "background context + bounded deadline" pattern so even a
// catastrophic failure leaves the audit trail honest. The Run loop
// keeps going on the next Receive.
func (h *Harness) recoverFromPanic(state *processState, task domain.Task) {
	r := recover()
	if r == nil {
		slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.recoverFromPanic.no_panic")
		return
	}
	slog.Error("agent harness panic", "cmd", harnessLogCmd,
		"operation", "agent.harness.Harness.recoverFromPanic", "task_id", task.ID,
		"cycle_id", state.cycleID, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
	bg, cancel := context.WithTimeout(context.Background(), h.opts.ShutdownAbortTimeout)
	defer cancel()
	if state.runningPhaseSeq > 0 {
		summary := PanicReason
		if _, err := h.store.CompletePhase(bg, store.CompletePhaseInput{
			CycleID:  state.cycleID,
			PhaseSeq: state.runningPhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			slog.Warn("agent harness panic CompletePhase failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.recoverFromPanic.complete_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			h.publish(task.ID, state.cycleID)
		}
		state.runningPhase = ""
		state.runningPhaseSeq = 0
	}
	if state.cycleStarted {
		if _, err := h.store.TerminateCycle(bg, state.cycleID, domain.CycleStatusFailed, PanicReason, domain.ActorAgent); err != nil {
			slog.Warn("agent harness panic TerminateCycle failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.recoverFromPanic.terminate_err",
				"cycle_id", state.cycleID, "err", err)
		} else {
			h.publish(task.ID, state.cycleID)
			h.recordRun(string(domain.CycleStatusFailed), h.runner.Name(), state.effectiveModel, state.startedAt)
		}
		state.cycleStarted = false
	}
	failed := domain.StatusFailed
	if _, err := h.store.Update(bg, task.ID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent harness panic task transition failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.recoverFromPanic.task_err",
				"task_id", task.ID, "err", err)
		}
	}
	h.cleanupCycleReports(state.cycleID, "panic")
}

// bestEffortFailTask is the cleanup path used when StartCycle itself
// failed (so there is no cycle row to terminate but the task is now
// `running` and would otherwise be re-enqueued forever by the
// reconcile loop). See docs/architecture.md "Lifecycle of one task".
func (h *Harness) bestEffortFailTask(ctx context.Context, taskID string) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.bestEffortFailTask",
		"task_id", taskID)
	failed := domain.StatusFailed
	if _, err := h.store.Update(ctx, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent harness bestEffortFailTask failed", "cmd", harnessLogCmd,
				"operation", "agent.harness.Harness.bestEffortFailTask.err",
				"task_id", taskID, "err", err)
		}
	}
}

// bestEffortTerminate closes a cycle that was opened but whose phase
// pipeline tripped before runner.Run; used when StartPhase or its
// follow-up writes failed. Best-effort: store errors are logged and
// swallowed, the startup sweep is the safety net.
func (h *Harness) bestEffortTerminate(ctx context.Context, state *processState, taskID string, status domain.CycleStatus, reason string) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.bestEffortTerminate",
		"cycle_id", state.cycleID, "status", string(status), "reason", reason)
	if state.runningPhaseSeq > 0 {
		summary := reason
		if _, err := h.store.CompletePhase(ctx, store.CompletePhaseInput{
			CycleID:  state.cycleID,
			PhaseSeq: state.runningPhaseSeq,
			Status:   domain.PhaseStatusFailed,
			Summary:  &summary,
			By:       domain.ActorAgent,
		}); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				slog.Warn("agent harness bestEffortTerminate CompletePhase failed",
					"cmd", harnessLogCmd,
					"operation", "agent.harness.Harness.bestEffortTerminate.complete_err",
					"cycle_id", state.cycleID, "err", err)
			}
		} else {
			h.publish(taskID, state.cycleID)
		}
		state.runningPhase = ""
		state.runningPhaseSeq = 0
	}
	if state.cycleStarted {
		if _, err := h.store.TerminateCycle(ctx, state.cycleID, status, reason, domain.ActorAgent); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				slog.Warn("agent harness bestEffortTerminate TerminateCycle failed",
					"cmd", harnessLogCmd,
					"operation", "agent.harness.Harness.bestEffortTerminate.terminate_err",
					"cycle_id", state.cycleID, "err", err)
			}
		} else {
			h.publish(taskID, state.cycleID)
			h.recordRun(string(status), h.runner.Name(), state.effectiveModel, state.startedAt)
		}
		state.cycleStarted = false
	}
	h.bestEffortFailTask(ctx, taskID)
	h.cleanupCycleReports(state.cycleID, "best_effort_terminate")
}
