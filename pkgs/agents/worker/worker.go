package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const workerLogCmd = "taskapi"

// Worker is the single-goroutine in-process consumer of the
// MemoryQueue (contract: docs/architecture.md). It handles queue
// admission and delegates cycle choreography to pkgs/agents/harness.
type Worker struct {
	store   *store.Store
	queue   *agents.MemoryQueue
	harness *harness.Harness
	opts    Options
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NewWorker constructs a Worker with sensible defaults applied to opts.
func NewWorker(st *store.Store, q *agents.MemoryQueue, r runner.Runner, opts Options) *Worker {
	if opts.ShutdownAbortTimeout <= 0 {
		opts.ShutdownAbortTimeout = DefaultShutdownAbortTimeout
	}
	h := harness.New(st, r, opts)
	return &Worker{store: st, queue: q, harness: h, opts: opts}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// CancelCurrentRun cancels the in-flight runner.Run, if any.
func (w *Worker) CancelCurrentRun() bool {
	if w == nil || w.harness == nil {
		return false
	}
	return w.harness.CancelCurrentRun()
}

// Run blocks on the queue and processes one task at a time until ctx cancels.
func (w *Worker) Run(ctx context.Context) error {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.Run")
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
				slog.Info("agent worker shutdown", "cmd", workerLogCmd,
					"operation", "agent.worker.Worker.Run.shutdown", "err", err)
				return nil
			}
			return fmt.Errorf("agent worker receive: %w", err)
		}
		w.processOne(ctx, task)
	}
}
