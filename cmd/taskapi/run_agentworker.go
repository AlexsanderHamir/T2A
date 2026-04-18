package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/internal/taskapiconfig"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// run_agentworker.go owns the optional in-process Cursor-CLI agent
// worker wiring: bounded ready-task queue, reconcile loop, startup
// orphan sweep, Cursor probe, SSE notifier adapter, and the
// shutdown-drain handle. Split off run_helpers.go per
// backend-engineering-bar.mdc §2 / §16.

// shutdownGraceAfterRunTimeout is the headroom added to
// T2A_AGENT_WORKER_RUN_TIMEOUT when waiting for Worker.Run to drain
// during shutdown. The extra slack covers the worker's own deferred
// best-effort writes (handleShutdownAfterRun) so they can land before
// the reconcile ctx and DB pool close.
const shutdownGraceAfterRunTimeout = 10 * time.Second

// agentWorkerStartupSweepTimeout bounds the one-shot
// SweepOrphanRunningCycles call we run before starting the worker.
// The sweep is best-effort housekeeping for cycle/phase rows left in
// 'running' by a previous crash; if it can't finish in this budget we
// log and continue so a slow DB doesn't block startup indefinitely
// (bar §16: avoid hardcoded literals — name the duration).
const agentWorkerStartupSweepTimeout = 30 * time.Second

// agentWorkerHandle bundles the per-process state for the optional
// agent worker. When the worker is disabled (the documented default),
// every field except waitDone is zero/nil and waitDone is a closed
// channel so shutdown sequencing stays uniform.
type agentWorkerHandle struct {
	worker       *worker.Worker
	cancelWorker context.CancelFunc
	waitDone     chan struct{}
	runTimeout   time.Duration
}

// drain blocks until Worker.Run returns or the bounded shutdown
// deadline trips. The deadline is RunTimeout plus a fixed grace so
// the worker's own best-effort post-cancel writes
// (handleShutdownAfterRun) can land before reconcile or the DB pool
// close. Closes the worker context first so the runner sees a
// cancelled ctx promptly.
func (h *agentWorkerHandle) drain() {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerHandle.drain")
	if h == nil || h.cancelWorker == nil {
		return
	}
	h.cancelWorker()
	deadline := h.runTimeout + shutdownGraceAfterRunTimeout
	select {
	case <-h.waitDone:
		slog.Info("agent worker drained", "cmd", cmdName, "operation", "taskapi.shutdown",
			"phase", "worker_done")
	case <-time.After(deadline):
		slog.Warn("agent worker drain timeout", "cmd", cmdName, "operation", "taskapi.shutdown",
			"phase", "worker_drain_timeout", "deadline", deadline.String())
	}
}

// startReadyTaskAgents wires the bounded ready-task queue, the
// reconcile loop, and (when T2A_AGENT_WORKER_ENABLED is truthy) the
// in-process Cursor CLI agent worker plus its startup orphan sweep.
// The returned cancel func tears down the reconcile goroutine; the
// worker handle is non-nil even when the worker is disabled so the
// shutdown path can call drain() unconditionally.
//
// Wiring order when the worker is enabled:
//  1. Probe `cursor --version` with a 5s budget; return error on failure.
//  2. Run worker.SweepOrphanRunningCycles once on the freshly opened
//     store so any cycle/phase rows stuck in 'running' from a previous
//     crash are closed before the new worker can race them.
//  3. Build the Cursor adapter using the probed version string.
//  4. Build the SSE notifier adapter that wraps hub.Publish.
//  5. Construct + Run the Worker on a child context derived from ctx.
//
// When the worker is disabled (default), only the queue + reconcile
// loop start — no probe, no sweep, no Cursor binary required.
func startReadyTaskAgents(ctx context.Context, taskStore *store.Store, hub *handler.SSEHub) (context.CancelFunc, *agents.MemoryQueue, *agentWorkerHandle, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.startReadyTaskAgents")
	qcap := taskapiconfig.UserTaskAgentQueueCap()
	agentQueue := agents.NewMemoryQueue(qcap)
	taskStore.SetReadyTaskNotifier(agentQueue)
	iv := taskapiconfig.UserTaskAgentReconcileInterval()
	slog.Info("ready task agent queue", "cmd", cmdName, "operation", "taskapi.agent_queue", "cap", qcap)
	slog.Info("ready task agent reconcile", "cmd", cmdName, "operation", "taskapi.agent_reconcile",
		"tick_interval", iv.String(), "periodic", iv > 0)

	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv)

	handle, err := startAgentWorkerIfEnabled(ctx, taskStore, agentQueue, hub)
	if err != nil {
		// Tear down the reconcile goroutine we just spawned so the
		// caller can safely return without leaking it; bar §16
		// forbids os.Exit outside main.go because it skips the log
		// file flush deferred in runTaskAPIService.
		reconcileCancel()
		return nil, nil, nil, err
	}
	return reconcileCancel, agentQueue, handle, nil
}

// startAgentWorkerIfEnabled returns a populated agentWorkerHandle when
// T2A_AGENT_WORKER_ENABLED is truthy, or a no-op handle (closed
// waitDone, nil cancel) otherwise. Failures inside the enabled branch
// (workdir missing, Cursor binary not usable) are surfaced as errors
// per the Stage 4 "fail loudly at startup" rule — but as a returned
// error rather than os.Exit so the caller's deferred log flush still
// runs (bar §16: log.Fatal/os.Exit outside main.go skips deferred
// cleanup).
func startAgentWorkerIfEnabled(ctx context.Context, taskStore *store.Store, agentQueue *agents.MemoryQueue, hub *handler.SSEHub) (*agentWorkerHandle, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.startAgentWorkerIfEnabled")
	enabled := taskapiconfig.AgentWorkerEnabled()
	runTimeout := taskapiconfig.AgentWorkerRunTimeout()
	workingDir := taskapiconfig.AgentWorkerWorkingDir()
	cursorBin := taskapiconfig.AgentWorkerCursorBin()
	if !enabled {
		slog.Info("agent worker config", "cmd", cmdName, "operation", "taskapi.agent_worker",
			"enabled", false, "runner", "", "cursor_bin", cursorBin,
			"cursor_version", "", "run_timeout_sec", int(runTimeout/time.Second),
			"working_dir", workingDir)
		closed := make(chan struct{})
		close(closed)
		return &agentWorkerHandle{waitDone: closed, runTimeout: runTimeout}, nil
	}

	if err := assertWorkingDirExists(workingDir); err != nil {
		return nil, fmt.Errorf("agent worker working dir %q not usable: %w", workingDir, err)
	}

	cursorVersion, err := cursor.Probe(ctx, cursorBin, cursor.DefaultProbeTimeout, nil)
	if err != nil {
		return nil, fmt.Errorf("cursor binary %q not usable: %w", cursorBin, err)
	}

	sweepCtx, cancelSweep := context.WithTimeout(ctx, agentWorkerStartupSweepTimeout)
	res, sweepErr := worker.SweepOrphanRunningCycles(sweepCtx, taskStore)
	cancelSweep()
	if sweepErr != nil {
		slog.Warn("agent worker startup sweep failed (continuing anyway)",
			"cmd", cmdName, "operation", "taskapi.agent_worker.sweep_err",
			"timeout_sec", int(agentWorkerStartupSweepTimeout/time.Second),
			"deadline_exceeded", errors.Is(sweepErr, context.DeadlineExceeded),
			"err", sweepErr)
	} else {
		slog.Info("agent worker startup sweep ok", "cmd", cmdName,
			"operation", "taskapi.agent_worker.sweep_ok",
			"cycles_aborted", res.CyclesAborted, "phases_failed", res.PhasesFailed,
			"tasks_failed", res.TasksFailed)
	}

	adapter := cursor.New(cursor.Options{
		BinaryPath: cursorBin,
		Version:    cursorVersion,
	})

	notifier := newCycleChangeSSEAdapter(hub)
	metrics := taskapi.RegisterAgentWorkerMetrics()
	w := worker.NewWorker(taskStore, agentQueue, adapter, worker.Options{
		RunTimeout: runTimeout,
		WorkingDir: workingDir,
		Notifier:   notifier,
		Metrics:    metrics,
	})

	workerCtx, cancelWorker := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Run(workerCtx); err != nil {
			slog.Error("agent worker exited with error", "cmd", cmdName,
				"operation", "taskapi.agent_worker.exit_err", "err", err)
		}
	}()

	slog.Info("agent worker config", "cmd", cmdName, "operation", "taskapi.agent_worker",
		"enabled", true, "runner", adapter.Name(), "cursor_bin", cursorBin,
		"cursor_version", cursorVersion, "run_timeout_sec", int(runTimeout/time.Second),
		"working_dir", workingDir)

	return &agentWorkerHandle{
		worker:       w,
		cancelWorker: cancelWorker,
		waitDone:     done,
		runTimeout:   runTimeout,
	}, nil
}

// assertWorkingDirExists is the fail-fast guard for
// T2A_AGENT_WORKER_WORKING_DIR. Returns an error when the path is
// missing or not a directory; the caller wraps with context and
// returns the error up the stack.
func assertWorkingDirExists(dir string) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.assertWorkingDirExists",
		"dir", dir)
	if dir == "" {
		return errors.New("working directory is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", dir)
	}
	return nil
}

// cycleChangeSSEAdapter implements worker.CycleChangeNotifier on top
// of the existing handler.SSEHub. The TaskCycleChanged event type and
// the SPA cache invalidation hook were added in
// EXECUTION-CYCLES-PLAN.md Stage 5/7; the Stage 4 worker is the first
// server-side publisher.
type cycleChangeSSEAdapter struct {
	hub *handler.SSEHub
}

func newCycleChangeSSEAdapter(hub *handler.SSEHub) *cycleChangeSSEAdapter {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.newCycleChangeSSEAdapter")
	return &cycleChangeSSEAdapter{hub: hub}
}

// PublishCycleChange satisfies worker.CycleChangeNotifier. Nil hub or
// blank ids are no-ops so the adapter is safe to wire even before the
// SSE listener is fully attached.
func (a *cycleChangeSSEAdapter) PublishCycleChange(taskID, cycleID string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.cycleChangeSSEAdapter.PublishCycleChange",
		"task_id", taskID, "cycle_id", cycleID)
	if a == nil || a.hub == nil || taskID == "" {
		return
	}
	a.hub.Publish(handler.TaskChangeEvent{
		Type:    handler.TaskCycleChanged,
		ID:      taskID,
		CycleID: cycleID,
	})
}
