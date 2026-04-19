package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// worker.go is the package's public surface: tunable constants,
// optional callback interfaces (CycleChangeNotifier — RunMetrics
// lives in metrics.go), Options, the Worker struct, NewWorker, and
// the Run loop. The per-task lifecycle (process.go), recovery paths
// (cleanup.go), and payload helpers (meta.go) live in sibling files
// per backend-engineering-bar.mdc §2 (the original single file had
// grown past the yellow tier).

const workerLogCmd = "taskapi"

// DefaultRunTimeout caps a single runner.Run invocation. Mirrored by
// T2A_AGENT_WORKER_RUN_TIMEOUT in Stage 4 wiring.
const DefaultRunTimeout = 5 * time.Minute

// DefaultShutdownAbortTimeout bounds the post-cancel best-effort writes
// (CompletePhase + TerminateCycle + Update task) that run on a
// non-cancelled background context after the parent ctx fires. See
// docs/AGENT-WORKER.md "Lifecycle of one task" for the shutdown path.
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
	// Metrics, when non-nil, receives one RecordRun call after every
	// TerminateCycle write (happy path, panic, shutdown abort, and
	// best-effort intermediate failures). Nil disables observation
	// (used in unit tests). cmd/taskapi wires a Prometheus adapter.
	Metrics RunMetrics
	// Clock, when non-nil, replaces time.Now().UTC() for duration
	// logging. Tests can stub a deterministic clock here.
	Clock func() time.Time
}

// Worker is the single-goroutine in-process consumer of the
// MemoryQueue (contract: docs/AGENT-WORKER.md). Construct it with
// NewWorker and drive it with Run; both methods are safe to call from
// one goroutine only (V1 explicitly does not run multiple workers in
// parallel).
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
			// Bar §4: never log AND return the same error. The cmd/taskapi
			// goroutine wrapping Run logs at Error level on non-nil return
			// (taskapi.agent_worker.exit_err) so we propagate without
			// double-logging here.
			return fmt.Errorf("agent worker receive: %w", err)
		}
		w.processOne(ctx, task)
	}
}

// publish notifies the SSE adapter (when wired). Nil notifier is the
// supported test default and produces no fan-out. Lives on the public
// surface because every per-task path (process.go, cleanup.go) needs
// it; keeping it here avoids a circular "which sibling owns publish?"
// debate.
func (w *Worker) publish(taskID, cycleID string) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.publish",
		"task_id", taskID, "cycle_id", cycleID)
	if w.options.Notifier == nil {
		return
	}
	w.options.Notifier.PublishCycleChange(taskID, cycleID)
}
