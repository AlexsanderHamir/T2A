package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const workerLogCmd = "taskapi"

// DefaultRunTimeout caps a single runner.Run invocation. Mirrored by
// T2A_AGENT_WORKER_RUN_TIMEOUT in Stage 4 wiring.
const DefaultRunTimeout = 5 * time.Minute

// DefaultShutdownAbortTimeout bounds the post-cancel best-effort writes
// (CompletePhase + TerminateCycle + Update task) that run on a
// non-cancelled background context after the parent ctx fires. Matches
// the Stage 3 design in docs/AGENT-WORKER-PLAN.md edge case #5.
const DefaultShutdownAbortTimeout = 5 * time.Second

// SkippedDiagnoseSummary is the canonical summary written on the no-op
// diagnose phase row. Pinned so the audit trail string is stable
// across worker invocations and refactors.
const SkippedDiagnoseSummary = "single-phase V1; diagnose deferred"

// PanicReason is the cycle/phase termination reason recorded when the
// recover path fires after a runner or store panic.
const PanicReason = "panic"

// ShutdownReason is the termination reason written when the parent
// context cancels mid-run.
const ShutdownReason = "shutdown"

// CycleChangeNotifier is the optional SSE seam. cmd/taskapi wires an
// adapter that calls hub.Publish(handler.TaskCycleChanged{...}); tests
// pass nil and every PublishCycleChange call becomes a no-op.
//
// Implementations MUST NOT block: the worker invokes PublishCycleChange
// synchronously after each cycle/phase write.
type CycleChangeNotifier interface {
	PublishCycleChange(taskID, cycleID string)
}

// Options bundles the per-Worker tunables. Zero values pick documented
// defaults so cmd/taskapi can construct a Worker without filling in
// every field.
type Options struct {
	// RunTimeout caps one runner.Run invocation. Defaults to
	// DefaultRunTimeout.
	RunTimeout time.Duration
	// ShutdownAbortTimeout bounds the best-effort cycle/phase/task
	// writes performed on a background context after the parent ctx is
	// cancelled. Defaults to DefaultShutdownAbortTimeout.
	ShutdownAbortTimeout time.Duration
	// WorkingDir is forwarded to runner.Request.WorkingDir verbatim.
	// V1 uses one shared directory across sequential runs; V2 will
	// move to per-cycle isolation (see Notes / followups).
	WorkingDir string
	// Notifier, when non-nil, receives one PublishCycleChange call after
	// each successful StartCycle / StartPhase / CompletePhase /
	// TerminateCycle. Nil disables fan-out (used in unit tests).
	Notifier CycleChangeNotifier
	// Clock, when non-nil, replaces time.Now().UTC() for duration
	// logging. Tests can stub a deterministic clock here.
	Clock func() time.Time
}

// Worker is the single-goroutine in-process consumer wired in
// docs/AGENT-WORKER-PLAN.md Stage 3. Construct it with NewWorker and
// drive it with Run; both methods are safe to call from one goroutine
// only (V1 explicitly does not run multiple workers in parallel).
type Worker struct {
	store   *store.Store
	queue   *agents.MemoryQueue
	runner  runner.Runner
	options Options
}

// NewWorker constructs a Worker with sensible defaults applied to opts.
// st, q, and r MUST be non-nil; callers that want a no-op runner pass
// runnerfake.New().
func NewWorker(st *store.Store, q *agents.MemoryQueue, r runner.Runner, opts Options) *Worker {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.NewWorker")
	if opts.RunTimeout <= 0 {
		opts.RunTimeout = DefaultRunTimeout
	}
	if opts.ShutdownAbortTimeout <= 0 {
		opts.ShutdownAbortTimeout = DefaultShutdownAbortTimeout
	}
	if opts.Clock == nil {
		opts.Clock = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &Worker{store: st, queue: q, runner: r, options: opts}
}

// Run blocks on the queue and processes one task at a time until ctx
// cancels. A cancelled parent ctx is the documented shutdown signal and
// produces a nil error return so the cmd/taskapi shutdown path does not
// log it as a failure. Any non-cancellation error returned by
// MemoryQueue.Receive (today: nil store, nil queue) is propagated.
func (w *Worker) Run(ctx context.Context) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.Run")
	if w == nil {
		return errors.New("agent worker: nil receiver")
	}
	if w.store == nil || w.queue == nil || w.runner == nil {
		return errors.New("agent worker: store, queue, and runner are required")
	}
	for {
		task, err := w.queue.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Info("agent worker shutdown", "cmd", workerLogCmd,
					"operation", "agent.worker.Worker.Run.shutdown", "err", err)
				return nil
			}
			slog.Error("agent worker receive failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.Run.receive_err", "err", err)
			return err
		}
		w.processOne(ctx, task)
	}
}

// processState records what the worker has written so far for a single
// task. The deferred panic-recovery and the shutdown branch use it to
// decide which cleanup writes are still needed.
type processState struct {
	cycleID         string
	cycleStarted    bool
	runningPhase    domain.Phase
	runningPhaseSeq int64
}

// processOne runs the worker's full per-task lifecycle. The function is
// intentionally long: keeping the happy path, the shutdown branch, and
// the deferred panic-recovery in one call site is what makes the
// "ack after terminate" ordering enforceable by reading top-to-bottom.
func (w *Worker) processOne(parentCtx context.Context, task domain.Task) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.processOne",
		"task_id", task.ID)
	startedAt := w.options.Clock()
	state := processState{}

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

	if parentCtx.Err() != nil {
		w.handleShutdownAfterRun(&state, task.ID)
		return
	}

	phaseStatus, cycleStatus, taskStatus, reason := classifyRunOutcome(runErr)
	if !w.completeExecutePhase(parentCtx, &state, cycle, execPhase, phaseStatus, result) {
		return
	}
	if !w.terminateCycle(parentCtx, &state, cycle.TaskID, cycleStatus, reason) {
		return
	}
	if !w.transitionTask(parentCtx, task.ID, taskStatus, "final_task_transition") {
		return
	}

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
// MetaJSON carries runner identity + prompt hash so the audit trail can
// distinguish runs by adapter version even before per-runner labels
// land in metrics.
func (w *Worker) startCycle(ctx context.Context, task *domain.Task, state *processState) (*domain.TaskCycle, bool) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.startCycle",
		"task_id", task.ID)
	meta := buildCycleMeta(w.runner, task.InitialPrompt)
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

// invokeRunner builds the Request, applies the per-run timeout, and
// returns whatever the runner produced. The returned error is the raw
// adapter error (typed via runner.Err* sentinels); classification is
// done by the caller so the shutdown branch can short-circuit it.
func (w *Worker) invokeRunner(parentCtx context.Context, task *domain.Task, cycle *domain.TaskCycle, exec *domain.TaskCyclePhase) (runner.Result, error) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.invokeRunner",
		"task_id", task.ID, "cycle_id", cycle.ID, "phase_seq", exec.PhaseSeq)
	runCtx, cancel := context.WithTimeout(parentCtx, w.options.RunTimeout)
	defer cancel()
	return w.runner.Run(runCtx, runner.Request{
		TaskID:     task.ID,
		AttemptSeq: cycle.AttemptSeq,
		Phase:      domain.PhaseExecute,
		Prompt:     task.InitialPrompt,
		WorkingDir: w.options.WorkingDir,
		Timeout:    w.options.RunTimeout,
	})
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
		// Whatever happened, the phase is no longer ours to close on
		// the happy path. Clear state so the panic-recovery branch
		// does not double-write.
		state.runningPhase = ""
		state.runningPhaseSeq = 0
		state.cycleStarted = false
		return false
	}
	state.runningPhase = ""
	state.runningPhaseSeq = 0
	w.publish(cycle.TaskID, cycle.ID)
	return true
}

// terminateCycle closes the cycle row and clears state so the recovery
// path is a no-op for already-terminal cycles.
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
	return true
}

// publish notifies the SSE adapter (when wired). Nil notifier is the
// supported test default and produces no fan-out.
func (w *Worker) publish(taskID, cycleID string) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.publish",
		"task_id", taskID, "cycle_id", cycleID)
	if w.options.Notifier == nil {
		return
	}
	w.options.Notifier.PublishCycleChange(taskID, cycleID)
}

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
// reconcile loop — edge case #2 in docs/AGENT-WORKER-PLAN.md).
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
		}
		state.cycleStarted = false
	}
	w.bestEffortFailTask(ctx, taskID)
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

// buildCycleMeta produces the JSON body written to TaskCycle.MetaJSON.
// The Stage-3 audit contract pins these three keys; adding more later
// is allowed but renames require a coordinated migration of the
// substrate's mirror payloads.
func buildCycleMeta(r runner.Runner, prompt string) []byte {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.buildCycleMeta",
		"runner", r.Name())
	out := map[string]string{
		"runner":         r.Name(),
		"runner_version": r.Version(),
		"prompt_hash":    sha256Hex(prompt),
	}
	b, err := json.Marshal(out)
	if err != nil {
		slog.Warn("agent worker meta marshal failed", "cmd", workerLogCmd,
			"operation", "agent.worker.buildCycleMeta.err", "err", err)
		return []byte("{}")
	}
	return b
}

// sha256Hex returns the lowercase hex SHA-256 of s. The worker writes
// this into MetaJSON.prompt_hash so the audit trail can correlate runs
// of the same prompt across replays without storing the prompt itself.
func sha256Hex(s string) string {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.sha256Hex",
		"len", len(s))
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// detailsBytes converts a runner.Result's free-form Details into the
// JSON object the store expects. nil/empty is normalised to "{}" so
// the kernel.NormalizeJSONObject guard never trips.
func detailsBytes(r runner.Result) []byte {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.detailsBytes",
		"len", len(r.Details))
	if len(r.Details) == 0 {
		return []byte("{}")
	}
	return r.Details
}
