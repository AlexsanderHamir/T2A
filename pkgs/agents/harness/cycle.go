package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	verifySnap      verificationSnapshot
	verifyAttempt   int
	verifyFeedback  string
	// previouslyPassed accumulates criterion verdicts that earlier
	// retry attempts proved passed. Keyed by criterion ID; carried in
	// memory across the retry loop so the next execute prompt can list
	// these items as "already verified, do not re-do" and the next
	// verify pass can short-circuit them. The atomic-decision contract
	// (docs/data-model.md "Worker verification loop") is preserved
	// because nothing here is committed to task_checklist_completions
	// until the cycle succeeds and applyVerifiedCompletions is called
	// with the union. On terminal failure the map is discarded.
	previouslyPassed map[string]criterionVerdict
	// startedAt is captured at processOne entry so every TerminateCycle
	// path (happy / panic / shutdown / best-effort) observes the same
	// wall-clock duration into the metrics histogram.
	startedAt time.Time
	// effectiveModel is captured in startCycle from
	// runner.MetricsLabeler (or runner.EffectiveModel as fallback)
	// so every TerminateCycle path emits the SAME model label into
	// the by-model Prometheus series — even if the operator edited
	// task.CursorModel between StartCycle and TerminateCycle.
	effectiveModel string
	// gitSnap holds execute-start anchors for commit ingest on success.
	gitSnap gitPhaseSnapshot
}

// Run drives the harness cycle body for one task already in StatusRunning.
// The worker owns queue admission (reload, readiness, ready→running) and
// ack ordering before calling Run.
func (h *Harness) Run(parentCtx context.Context, task *domain.Task) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.Run",
		"task_id", task.ID)
	startedAt := h.opts.Clock()
	state := processState{startedAt: startedAt, previouslyPassed: map[string]criterionVerdict{}}

	defer h.recoverFromPanic(&state, *task)

	cycle, ok := h.startCycle(parentCtx, task, &state)
	if !ok {
		h.bestEffortFailTask(parentCtx, task.ID)
		return
	}

	state.verifySnap, _ = h.loadVerificationSnapshot(parentCtx, task.ID)
	h.runCycleLoop(parentCtx, task, cycle, &state, cycleLoopOpts{})
}

// transitionTask flips the task to next; returns false on any store
// error (including ErrNotFound when the task was deleted mid-cycle).
func (h *Harness) transitionTask(ctx context.Context, taskID string, next domain.Status, op string) bool {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.transitionTask",
		"task_id", taskID, "next", string(next), "op", op)
	if _, err := h.store.Update(ctx, taskID, store.UpdateTaskInput{Status: &next}, domain.ActorAgent); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent harness task transition failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.transitionTask.err",
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
// Runner-specific metadata (e.g. model intent/effective) comes from
// the CycleMetaProvider interface; metric model labels from
// MetricsLabeler. Both may produce "" and that empty string is the
// truth, not a placeholder.
func (h *Harness) startCycle(ctx context.Context, task *domain.Task, state *processState) (*domain.TaskCycle, bool) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.startCycle",
		"task_id", task.ID)
	req := runner.Request{
		TaskID:      task.ID,
		Phase:       domain.PhaseExecute,
		Prompt:      task.InitialPrompt,
		WorkingDir:  h.opts.WorkingDir,
		CursorModel: task.CursorModel,
	}
	meta := buildCycleMeta(h.runner, task.InitialPrompt, req)
	cycle, err := h.store.StartCycle(ctx, store.StartCycleInput{
		TaskID:      task.ID,
		TriggeredBy: domain.ActorAgent,
		Meta:        meta,
	})
	if err != nil {
		slog.Warn("agent harness StartCycle failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.startCycle.err", "task_id", task.ID, "err", err)
		return nil, false
	}
	state.cycleID = cycle.ID
	state.cycleStarted = true
	if ml, ok := h.runner.(runner.MetricsLabeler); ok {
		labels := ml.MetricsLabels(req)
		state.effectiveModel = labels["model"]
	} else {
		state.effectiveModel = h.runner.EffectiveModel(req)
	}
	h.publish(task.ID, cycle.ID)
	return cycle, true
}

// startExecutePhase opens the execute phase row that wraps runner.Run.
// state is updated so the panic-recovery and shutdown branches can find
// the phase to close out.
func (h *Harness) startExecutePhase(ctx context.Context, cycle *domain.TaskCycle, state *processState) (*domain.TaskCyclePhase, bool) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.startExecutePhase",
		"cycle_id", cycle.ID)
	exec, err := h.store.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		slog.Warn("agent harness StartPhase(execute) failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.startExecutePhase.err",
			"cycle_id", cycle.ID, "err", err)
		return nil, false
	}
	state.runningPhase = domain.PhaseExecute
	state.runningPhaseSeq = exec.PhaseSeq
	h.publish(cycle.TaskID, cycle.ID)
	return exec, true
}

func (h *Harness) invokeRunnerWithTask(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, exec *domain.TaskCyclePhase) (runner.Result, error) {
	return h.invokeRunner(parentCtx, task, cycle, exec)
}

// invokeRunner builds the Request, applies the per-run timeout (if any),
// publishes the cancel func so an operator can interrupt the run, and
// returns whatever the runner produced. <=0 RunTimeout means "no cap":
// the run can only be interrupted by the parent ctx (process shutdown)
// or CancelCurrentRun (operator-initiated). The returned error is the
// raw adapter error (typed via runner.Err* sentinels); classification
// is done by the caller so the shutdown branch can short-circuit it.
func (h *Harness) invokeRunner(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, exec *domain.TaskCyclePhase) (runner.Result, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.invokeRunner",
		"task_id", task.ID, "cycle_id", cycle.ID, "phase_seq", exec.PhaseSeq,
		"run_timeout_ns", int64(h.opts.RunTimeout))
	runCtx, cancel := withOptionalRunTimeout(parentCtx, h.opts.RunTimeout)
	defer cancel()
	projectContext, err := h.selectedProjectContext(runCtx, task, cycle)
	if err != nil {
		details, _ := json.Marshal(map[string]string{"error": err.Error()})
		return runner.NewResult(domain.PhaseStatusFailed, "project context selection failed", details, ""), fmt.Errorf("project context: %w: %v", runner.ErrInvalidOutput, err)
	}
	h.setCurrentRunCancel(cancel)
	defer h.setCurrentRunCancel(nil)
	return h.runner.Run(runCtx, runner.Request{
		TaskID:      task.ID,
		AttemptSeq:  cycle.AttemptSeq,
		Phase:       domain.PhaseExecute,
		Prompt:      promptWithProjectContext(task.InitialPrompt, projectContext.Text),
		WorkingDir:  h.opts.WorkingDir,
		Timeout:     h.opts.RunTimeout,
		CursorModel: task.CursorModel,
		OnProgress: func(ev runner.ProgressEvent) {
			h.persistProgress(runCtx, task.ID, cycle.ID, exec.PhaseSeq, ev)
			h.publishProgress(task.ID, cycle.ID, exec.PhaseSeq, ev)
		},
	})
}

func (h *Harness) persistProgress(ctx context.Context, taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.persistProgress",
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
			slog.Warn("agent harness progress payload marshal failed",
				"cmd", harnessLogCmd, "operation", "agent.harness.Harness.persistProgress.marshal_err",
				"task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq, "err", err)
			payload = []byte("{}")
		}
	}
	if _, err := h.store.AppendCycleStreamEvent(ctx, store.AppendCycleStreamEventInput{
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
		slog.Warn("agent harness progress persistence failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.persistProgress.err",
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
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.withOptionalRunTimeout",
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
func (h *Harness) completeExecutePhase(ctx context.Context, state *processState, cycle *domain.TaskCycle, exec *domain.TaskCyclePhase, status domain.PhaseStatus, result runner.Result, phaseDetails []byte) bool {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.completeExecutePhase",
		"cycle_id", cycle.ID, "phase_seq", exec.PhaseSeq, "status", string(status))
	details := phaseDetails
	if details == nil {
		details = detailsBytes(result)
	}
	in := store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: exec.PhaseSeq,
		Status:   status,
		Details:  details,
		By:       domain.ActorAgent,
	}
	if result.Summary != "" {
		s := result.Summary
		in.Summary = &s
	}
	if _, err := h.store.CompletePhase(ctx, in); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent harness CompletePhase(execute) failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.completeExecutePhase.err",
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
	h.publish(cycle.TaskID, cycle.ID)
	return true
}

// terminateCycle closes the cycle row and clears state so the recovery
// path is a no-op for already-terminal cycles. Records one metrics
// observation on success so cmd/taskapi's Prometheus counter +
// histogram see the happy-path attempt outcome.
func (h *Harness) terminateCycle(ctx context.Context, state *processState, taskID string, status domain.CycleStatus, reason string) bool {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.terminateCycle",
		"cycle_id", state.cycleID, "status", string(status), "reason", reason)
	if state.cycleID == "" {
		return true
	}
	if _, err := h.store.TerminateCycle(ctx, state.cycleID, status, reason, domain.ActorAgent); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent harness TerminateCycle failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.terminateCycle.err",
			"cycle_id", state.cycleID, "err", err)
		state.cycleStarted = false
		return false
	}
	state.cycleStarted = false
	h.publish(taskID, state.cycleID)
	h.recordRun(string(status), h.runner.Name(), state.effectiveModel, state.startedAt)
	h.observeVerifyRetries(state.verifyAttempt)
	// GC the worker-managed scratch dir for this cycle. Idempotent
	// against a missing dir; logged at Debug because operators rarely
	// care unless cleanup itself errors. Closes the unbounded-disk-
	// growth gap that existed when files were written under RepoRoot/.t2a.
	if err := cleanupReportDir(h.opts.ReportDir, state.cycleID); err != nil {
		slog.Warn("agent harness cleanupReportDir failed",
			"cmd", harnessLogCmd, "operation", "agent.harness.Harness.terminateCycle.cleanup_err",
			"cycle_id", state.cycleID, "report_dir", h.opts.ReportDir, "err", err)
	}
	return true
}

// classifyRunOutcome maps runner.Run's (Result, error) tuple to the
// triplet of phase status, cycle status, task status, plus a reason
// string that lands in the cycle's terminal mirror. Recoverable
// adapter failures (timeout, non-zero exit, invalid output) all map to
// failed; unexpected errors collapse to the same bucket so the worker
// is conservative about silent successes.
func classifyRunOutcome(err error) (domain.PhaseStatus, domain.CycleStatus, domain.Status, string) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.classifyRunOutcome",
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
