package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// process.go owns the happy-path per-task lifecycle: dequeue → reload
// → transition → cycle/phase pipeline → terminate. Recovery and
// shutdown branches live in cleanup.go; payload helpers
// (buildCycleMeta / detailsBytes) live in meta.go.

// processState records what the worker has written so far for a single
// task. The deferred panic-recovery and the shutdown branch use it to
// decide which cleanup writes are still needed.
type processState struct {
	cycleID         string
	cycleStarted    bool
	runningPhase    domain.Phase
	runningPhaseSeq int64
	// startedAt is captured at processOne entry so every TerminateCycle
	// path (happy / panic / shutdown / best-effort) observes the same
	// wall-clock duration into the metrics histogram.
	startedAt time.Time
	// effectiveModel is captured in startCycle from
	// runner.EffectiveModel(req) so every TerminateCycle path
	// (happy / panic / shutdown / best-effort) emits the SAME model
	// label into the by-model Prometheus series the cycle's
	// MetaJSON recorded — even if the operator edited
	// task.CursorModel between StartCycle and TerminateCycle.
	// Empty string is the truthful "no model configured" value;
	// the metrics adapter renders it as the empty label rather
	// than substituting a synthetic default.
	effectiveModel string
}

// processOne runs the worker's full per-task lifecycle. The function is
// intentionally long: keeping the happy path, the shutdown branch, and
// the deferred panic-recovery in one call site is what makes the
// "ack after terminate" ordering enforceable by reading top-to-bottom.
func (w *Worker) processOne(parentCtx context.Context, task domain.Task) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.processOne",
		"task_id", task.ID)
	startedAt := w.options.Clock()
	state := processState{startedAt: startedAt}

	// Defer order is LIFO: ack runs LAST so the queue-pending guard
	// holds for the entire processOne body, including the deferred
	// recovery writes. AckAfterRecv runs even on early returns and on
	// recovered panics so a single bad task cannot wedge the queue.
	defer w.queue.AckAfterRecv(task.ID)
	defer w.recoverFromPanic(&state, task)

	fresh, ok := w.reloadTask(parentCtx, task.ID)
	if !ok {
		return
	}
	if fresh.Status != domain.StatusReady {
		slog.Warn("stale task at dequeue", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.processOne.stale", "task_id", task.ID,
			"status", string(fresh.Status))
		return
	}
	if !w.transitionTask(parentCtx, task.ID, domain.StatusRunning, "transition_to_running") {
		return
	}

	cycle, ok := w.startCycle(parentCtx, fresh, &state)
	if !ok {
		w.bestEffortFailTask(parentCtx, task.ID)
		return
	}

	if !w.runSkippedDiagnose(parentCtx, cycle, &state) {
		w.bestEffortTerminate(parentCtx, &state, task.ID, domain.CycleStatusFailed, "diagnose_phase_write_failed")
		return
	}

	execPhase, ok := w.startExecutePhase(parentCtx, cycle, &state)
	if !ok {
		w.bestEffortTerminate(parentCtx, &state, task.ID, domain.CycleStatusFailed, "execute_phase_start_failed")
		return
	}

	result, runErr := w.invokeRunner(parentCtx, fresh, cycle, execPhase)
	operatorCancelled := w.consumeOperatorCancel()

	if parentCtx.Err() != nil {
		w.handleShutdownAfterRun(&state, task.ID)
		return
	}

	phaseStatus, cycleStatus, taskStatus, reason := classifyRunOutcome(runErr)
	if operatorCancelled {
		// The runner observed a cancelled ctx and surfaced an
		// ErrTimeout-shaped error. Override the classifier so the
		// audit trail records why the cycle ended (the operator hit
		// "Cancel current run") rather than implying a per-run
		// timeout fired or the runner produced bad output.
		reason = CancelledByOperatorReason
		if result.Summary == "" || strings.HasPrefix(result.Summary, "cursor: timeout") {
			result.Summary = "cancelled by operator"
		}
	}
	// On a successful run, mark the task's checklist criteria as
	// done before closing the phase so the subsequent
	// transitionTask(StatusDone) passes ValidateCanMarkDoneInTx
	// (which requires a completion row for every inherited item).
	// Without this the task would be silently stuck in `running` for
	// any task that has done-criteria, even though the runner
	// succeeded — see process.go::transitionTask, which only logs a
	// warning and returns false on the validation reject. If the
	// completion writes themselves fail we degrade the whole run to
	// failed so the cycle / task end up consistent rather than half
	// transitioned.
	if runErr == nil && !operatorCancelled {
		if err := w.completeChecklistOnSuccess(parentCtx, task.ID); err != nil {
			slog.Warn("agent worker checklist completion failed",
				"cmd", workerLogCmd,
				"operation", "agent.worker.Worker.processOne.checklist_err",
				"task_id", task.ID, "err", err)
			phaseStatus = domain.PhaseStatusFailed
			cycleStatus = domain.CycleStatusFailed
			taskStatus = domain.StatusFailed
			reason = checklistCompletionFailedReason
		}
	}
	if !w.completeExecutePhase(parentCtx, &state, cycle, execPhase, phaseStatus, result) {
		// CompletePhase failed (phase row indeterminate). The cycle
		// row is still ours to close — without this terminate the
		// cycle/task rows are orphaned in `running` until the next
		// process restart triggers SweepOrphanRunningCycles. We force
		// the cycle to `failed` regardless of the runner's outcome:
		// even a successful run is a write failure from the audit
		// trail's perspective if we could not persist the phase
		// terminal status.
		w.bestEffortTerminate(parentCtx, &state, task.ID, domain.CycleStatusFailed, completePhaseFailedReason)
		return
	}
	if !w.terminateCycle(parentCtx, &state, cycle.TaskID, cycleStatus, reason) {
		return
	}
	if !w.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition") {
		return
	}
	// Re-publish the cycle frame *after* the final task status row has
	// been written. terminateCycle (above) already emitted one publish,
	// but the order is: cycle row writes -> SSE publish -> task row
	// status transitions. The SPA's invalidation handler refetches the
	// task on the publish, races the in-flight transitionTask, and
	// usually wins — leaving the open task page stuck on `running` even
	// though the work is done. There are no further SSE frames after
	// the transition (transitionTask doesn't publish, and the worker
	// goes idle), so the stale row sticks until the user refreshes.
	//
	// One extra publish here is the cheap fix: the frontend coalesces
	// both publishes into a single flush (SSE_INVALIDATE_WINDOW_MS in
	// useTaskEventStream is 900ms; the gap between terminateCycle and
	// transitionTask is well under that), so the user sees one
	// invalidation that fetches the post-transition row. The cycle ID
	// is reused intentionally — the frame is identifying *which* task
	// changed, not the cycle's own state, and the SPA only cares about
	// the task scope for invalidation.
	w.publish(task.ID, cycle.ID)

	slog.Info("agent worker run complete", "cmd", workerLogCmd,
		"operation", "agent.worker.Worker.processOne.summary",
		"task_id", task.ID, "cycle_id", cycle.ID, "attempt_seq", cycle.AttemptSeq,
		"terminal_cycle_status", string(cycleStatus), "task_status", string(taskStatus),
		"runner", w.runner.Name(), "runner_version", w.runner.Version(),
		"duration_ms", w.options.Clock().Sub(startedAt).Milliseconds())
}

// reloadTask fetches the freshest task row from the store. ok==false
// means the caller should bail (and AckAfterRecv via the deferred path).
func (w *Worker) reloadTask(ctx context.Context, taskID string) (*domain.Task, bool) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.reloadTask",
		"task_id", taskID)
	fresh, err := w.store.Get(ctx, taskID)
	if err == nil {
		return fresh, true
	}
	if errors.Is(err, domain.ErrNotFound) {
		slog.Info("task vanished before dequeue processing", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.reloadTask.not_found", "task_id", taskID)
		return nil, false
	}
	slog.Warn("agent worker reload failed", "cmd", workerLogCmd,
		"operation", "agent.worker.Worker.reloadTask.err", "task_id", taskID, "err", err)
	return nil, false
}

// transitionTask flips the task to next; returns false on any store
// error (including ErrNotFound when the task was deleted mid-cycle).
func (w *Worker) transitionTask(ctx context.Context, taskID string, next domain.Status, op string) bool {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.transitionTask",
		"task_id", taskID, "next", string(next), "op", op)
	if _, err := w.store.Update(ctx, taskID, store.UpdateTaskInput{Status: &next}, domain.ActorAgent); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent worker task transition failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.transitionTask.err",
			"task_id", taskID, "next", string(next), "op", op, "err", err)
		return false
	}
	return true
}

// startCycle writes the StartCycle row and updates state on success.
// MetaJSON carries runner identity, prompt hash, AND the operator's
// model intent + the runner's resolved effective model (Phase 1a-ii of
// the per-task runner/model attribution plan) so the audit trail and
// observability slice-and-dice can distinguish runs by adapter
// version, intent, and effective model — without depending on runtime
// metric scrapes.
//
// The Request is the same shape invokeRunner builds later (sans
// per-run timeout, which is irrelevant to attribution). Intent is
// recorded verbatim from task.CursorModel; the runner resolves
// effective via Runner.EffectiveModel — both may be "" and that
// empty string is the truth, not a placeholder.
func (w *Worker) startCycle(ctx context.Context, task *domain.Task, state *processState) (*domain.TaskCycle, bool) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.startCycle",
		"task_id", task.ID)
	req := runner.Request{
		TaskID:      task.ID,
		Phase:       domain.PhaseExecute,
		Prompt:      task.InitialPrompt,
		WorkingDir:  w.options.WorkingDir,
		CursorModel: task.CursorModel,
	}
	meta := buildCycleMeta(w.runner, task.InitialPrompt, req)
	cycle, err := w.store.StartCycle(ctx, store.StartCycleInput{
		TaskID:      task.ID,
		TriggeredBy: domain.ActorAgent,
		Meta:        meta,
	})
	if err != nil {
		slog.Warn("agent worker StartCycle failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.startCycle.err", "task_id", task.ID, "err", err)
		return nil, false
	}
	state.cycleID = cycle.ID
	state.cycleStarted = true
	state.effectiveModel = w.runner.EffectiveModel(req)
	w.publish(task.ID, cycle.ID)
	return cycle, true
}

// runSkippedDiagnose writes the no-op diagnose row required by the
// substrate's "first phase must be diagnose" rule and immediately marks
// it skipped. Returns false on any store error so the caller can clean
// up the parent cycle.
func (w *Worker) runSkippedDiagnose(ctx context.Context, cycle *domain.TaskCycle, state *processState) bool {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.runSkippedDiagnose",
		"cycle_id", cycle.ID)
	diag, err := w.store.StartPhase(ctx, cycle.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		slog.Warn("agent worker StartPhase(diagnose) failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.runSkippedDiagnose.start_err",
			"cycle_id", cycle.ID, "err", err)
		return false
	}
	state.runningPhase = domain.PhaseDiagnose
	state.runningPhaseSeq = diag.PhaseSeq
	w.publish(cycle.TaskID, cycle.ID)

	summary := SkippedDiagnoseSummary
	if _, err := w.store.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: diag.PhaseSeq,
		Status:   domain.PhaseStatusSkipped,
		Summary:  &summary,
		By:       domain.ActorAgent,
	}); err != nil {
		slog.Warn("agent worker CompletePhase(diagnose) failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.runSkippedDiagnose.complete_err",
			"cycle_id", cycle.ID, "err", err)
		return false
	}
	state.runningPhase = ""
	state.runningPhaseSeq = 0
	w.publish(cycle.TaskID, cycle.ID)
	return true
}

// startExecutePhase opens the execute phase row that wraps runner.Run.
// state is updated so the panic-recovery and shutdown branches can find
// the phase to close out.
func (w *Worker) startExecutePhase(ctx context.Context, cycle *domain.TaskCycle, state *processState) (*domain.TaskCyclePhase, bool) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.startExecutePhase",
		"cycle_id", cycle.ID)
	exec, err := w.store.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		slog.Warn("agent worker StartPhase(execute) failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.startExecutePhase.err",
			"cycle_id", cycle.ID, "err", err)
		return nil, false
	}
	state.runningPhase = domain.PhaseExecute
	state.runningPhaseSeq = exec.PhaseSeq
	w.publish(cycle.TaskID, cycle.ID)
	return exec, true
}

// invokeRunner builds the Request, applies the per-run timeout (if any),
// publishes the cancel func so an operator can interrupt the run, and
// returns whatever the runner produced. <=0 RunTimeout means "no cap":
// the run can only be interrupted by the parent ctx (process shutdown)
// or CancelCurrentRun (operator-initiated). The returned error is the
// raw adapter error (typed via runner.Err* sentinels); classification
// is done by the caller so the shutdown branch can short-circuit it.
func (w *Worker) invokeRunner(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, exec *domain.TaskCyclePhase) (runner.Result, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.invokeRunner",
		"task_id", task.ID, "cycle_id", cycle.ID, "phase_seq", exec.PhaseSeq,
		"run_timeout_ns", int64(w.options.RunTimeout))
	runCtx, cancel := withOptionalRunTimeout(parentCtx, w.options.RunTimeout)
	defer cancel()
	projectContext, err := w.selectedProjectContext(runCtx, task, cycle)
	if err != nil {
		details, _ := json.Marshal(map[string]string{"error": err.Error()})
		return runner.NewResult(domain.PhaseStatusFailed, "project context selection failed", details, ""), fmt.Errorf("project context: %w: %v", runner.ErrInvalidOutput, err)
	}
	w.setCurrentRunCancel(cancel)
	defer w.setCurrentRunCancel(nil)
	return w.runner.Run(runCtx, runner.Request{
		TaskID:      task.ID,
		AttemptSeq:  cycle.AttemptSeq,
		Phase:       domain.PhaseExecute,
		Prompt:      promptWithProjectContext(task.InitialPrompt, projectContext.Text),
		WorkingDir:  w.options.WorkingDir,
		Timeout:     w.options.RunTimeout,
		CursorModel: task.CursorModel,
		OnProgress: func(ev runner.ProgressEvent) {
			w.persistProgress(runCtx, task.ID, cycle.ID, exec.PhaseSeq, ev)
			w.publishProgress(task.ID, cycle.ID, exec.PhaseSeq, ev)
		},
	})
}

func (w *Worker) persistProgress(ctx context.Context, taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.persistProgress",
		"task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq,
		"kind", ev.Kind, "subtype", ev.Subtype)
	if ev.Kind == "" {
		return
	}
	payload := ev.Payload
	if len(payload) == 0 {
		var err error
		payload, err = json.Marshal(ev)
		if err != nil {
			slog.Warn("agent worker progress payload marshal failed",
				"cmd", workerLogCmd, "operation", "agent.worker.Worker.persistProgress.marshal_err",
				"task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq, "err", err)
			payload = []byte("{}")
		}
	}
	if _, err := w.store.AppendCycleStreamEvent(ctx, store.AppendCycleStreamEventInput{
		TaskID:   taskID,
		CycleID:  cycleID,
		PhaseSeq: phaseSeq,
		Source:   "cursor",
		Kind:     ev.Kind,
		Subtype:  ev.Subtype,
		Message:  ev.Message,
		Tool:     ev.Tool,
		Payload:  payload,
	}); err != nil {
		slog.Warn("agent worker progress persistence failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.persistProgress.err",
			"task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq,
			"kind", ev.Kind, "err", err)
	}
}

// withOptionalRunTimeout returns a derived context that either inherits
// only the parent (no cap) or carries an additional WithTimeout. Pulled
// out so the no-cap path is a single function call rather than a branch
// inside invokeRunner. The returned cancel func MUST be called either
// directly (defer) or via CancelCurrentRun.
func withOptionalRunTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.withOptionalRunTimeout",
		"timeout_ns", int64(d))
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}

// completeExecutePhase persists the runner's outcome on the execute
// phase row. Errors from the store are logged and reported back so the
// caller can stop the pipeline (a missing row usually means the task
// was deleted mid-cycle).
func (w *Worker) completeExecutePhase(ctx context.Context, state *processState, cycle *domain.TaskCycle, exec *domain.TaskCyclePhase, status domain.PhaseStatus, result runner.Result) bool {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.completeExecutePhase",
		"cycle_id", cycle.ID, "phase_seq", exec.PhaseSeq, "status", string(status))
	in := store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: exec.PhaseSeq,
		Status:   status,
		Details:  detailsBytes(result),
		By:       domain.ActorAgent,
	}
	if result.Summary != "" {
		s := result.Summary
		in.Summary = &s
	}
	if _, err := w.store.CompletePhase(ctx, in); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent worker CompletePhase(execute) failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.completeExecutePhase.err",
			"cycle_id", cycle.ID, "phase_seq", exec.PhaseSeq, "err", err)
		// The phase row is in an indeterminate state (either still
		// running, already terminal, or vanished). Clear the phase
		// pointer so bestEffortTerminate's CompletePhase retry is
		// skipped — but leave cycleStarted=true so the cycle row
		// itself still gets terminated, otherwise the cycle row is
		// orphaned in `running` and the task row is orphaned in
		// `running`, requiring the startup orphan sweep to clean up
		// (see meta.go::detailsBytes for the historical context). The
		// deferred recoverFromPanic only acts on actual panics, so
		// leaving cycleStarted=true here cannot cause a double
		// TerminateCycle on the happy-error path.
		state.runningPhase = ""
		state.runningPhaseSeq = 0
		return false
	}
	state.runningPhase = ""
	state.runningPhaseSeq = 0
	w.publish(cycle.TaskID, cycle.ID)
	return true
}

// terminateCycle closes the cycle row and clears state so the recovery
// path is a no-op for already-terminal cycles. Records one metrics
// observation on success so cmd/taskapi's Prometheus counter +
// histogram see the happy-path attempt outcome.
func (w *Worker) terminateCycle(ctx context.Context, state *processState, taskID string, status domain.CycleStatus, reason string) bool {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.terminateCycle",
		"cycle_id", state.cycleID, "status", string(status), "reason", reason)
	if state.cycleID == "" {
		return true
	}
	if _, err := w.store.TerminateCycle(ctx, state.cycleID, status, reason, domain.ActorAgent); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent worker TerminateCycle failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.terminateCycle.err",
			"cycle_id", state.cycleID, "err", err)
		state.cycleStarted = false
		return false
	}
	state.cycleStarted = false
	w.publish(taskID, state.cycleID)
	w.recordRun(string(status), w.runner.Name(), state.effectiveModel, state.startedAt)
	return true
}

// completeChecklistOnSuccess marks every still-open checklist item on
// taskID as done. The agent is the only actor permitted to toggle
// completion (store/internal/checklist.SetDone), and a successful run
// is the implicit signal that the criteria the user attached to the
// task have been satisfied. Items already done are skipped so the call
// is idempotent across re-runs (a future retry path can call this
// without producing duplicate audit events).
//
// Tasks with no checklist items short-circuit to nil — the e2e happy
// path tests in worker_test.go exercise this case and must not regress.
func (w *Worker) completeChecklistOnSuccess(ctx context.Context, taskID string) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.completeChecklistOnSuccess",
		"task_id", taskID)
	items, err := w.store.ListChecklistForSubject(ctx, taskID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Done {
			continue
		}
		if err := w.store.SetChecklistItemDone(ctx, taskID, it.ID, true, domain.ActorAgent); err != nil {
			return err
		}
	}
	return nil
}

// classifyRunOutcome maps runner.Run's (Result, error) tuple to the
// triplet of phase status, cycle status, task status, plus a reason
// string that lands in the cycle's terminal mirror. Recoverable
// adapter failures (timeout, non-zero exit, invalid output) all map to
// failed; unexpected errors collapse to the same bucket so the worker
// is conservative about silent successes.
func classifyRunOutcome(err error) (domain.PhaseStatus, domain.CycleStatus, domain.Status, string) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.classifyRunOutcome",
		"err", err)
	if err == nil {
		return domain.PhaseStatusSucceeded, domain.CycleStatusSucceeded, domain.StatusDone, ""
	}
	switch {
	case errors.Is(err, runner.ErrTimeout):
		return domain.PhaseStatusFailed, domain.CycleStatusFailed, domain.StatusFailed, "runner_timeout"
	case errors.Is(err, runner.ErrNonZeroExit):
		return domain.PhaseStatusFailed, domain.CycleStatusFailed, domain.StatusFailed, "runner_non_zero_exit"
	case errors.Is(err, runner.ErrInvalidOutput):
		return domain.PhaseStatusFailed, domain.CycleStatusFailed, domain.StatusFailed, "runner_invalid_output"
	default:
		return domain.PhaseStatusFailed, domain.CycleStatusFailed, domain.StatusFailed, "runner_error"
	}
}
