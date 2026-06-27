package worker

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// Worker is an in-process consumer of the MemoryQueue (contract:
// docs/architecture.md). Pool mode runs N Workers sharing one queue and gate.
type Worker struct {
	store   *store.Store
	queue   *agents.MemoryQueue
	harness *harness.Harness
	opts    Options
	gitSvc  gitwork.Service
	gate    *WorktreeGate
}

// NewWorker constructs a Worker with sensible defaults applied to opts.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func NewWorker(st *store.Store, q *agents.MemoryQueue, r runner.Runner, opts Options) *Worker {
	return NewWorkerWithGate(st, q, r, opts, &WorktreeGate{})
}

// NewWorkerWithGate constructs a Worker that shares gate with a pool.
func NewWorkerWithGate(st *store.Store, q *agents.MemoryQueue, r runner.Runner, opts Options, gate *WorktreeGate) *Worker {
	if opts.ShutdownAbortTimeout <= 0 {
		opts.ShutdownAbortTimeout = DefaultShutdownAbortTimeout
	}
	if gate == nil {
		gate = &WorktreeGate{}
	}
	h := harness.New(st, r, opts)
	return &Worker{store: st, queue: q, harness: h, opts: opts, gate: gate}
}

// CancelCurrentRun cancels the in-flight runner.Run, if any.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (w *Worker) CancelCurrentRun() bool {
	if w == nil || w.harness == nil {
		return false
	}
	return w.harness.CancelCurrentRun()
}

// Run blocks on the queue and processes one task at a time until ctx cancels.
func (w *Worker) Run(ctx context.Context) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.Worker.Run")
	if w == nil {
		return errors.New("agent worker: nil receiver")
	}
	if w.store == nil || w.queue == nil || w.harness == nil {
		return errors.New("agent worker: store, queue, and harness are required")
	}
	for {
		task, err := w.queue.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Info("agent worker shutdown", "cmd", calltrace.LogCmd,
					"operation", "agent.worker.Worker.Run.shutdown", "err", err)
				return nil
			}
			return fmt.Errorf("agent worker receive: %w", err)
		}
		w.processOne(ctx, task)
	}
}
