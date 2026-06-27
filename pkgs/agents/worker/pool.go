package worker

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// Pool runs N queue consumers sharing one MemoryQueue and WorktreeGate.
type Pool struct {
	store *store.Store
	queue *agents.MemoryQueue
	gate  *WorktreeGate
	slots []*Worker
}

// NewPool constructs a worker pool with one harness per slot.
func NewPool(st *store.Store, q *agents.MemoryQueue, r runner.Runner, opts Options, concurrency int) *Pool {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 32 {
		concurrency = 32
	}
	gate := &WorktreeGate{}
	slots := make([]*Worker, concurrency)
	for i := range slots {
		slots[i] = NewWorkerWithGate(st, q, r, opts, gate)
	}
	return &Pool{store: st, queue: q, gate: gate, slots: slots}
}

// Run blocks until ctx cancels. Each slot goroutine receives from the shared queue.
func (p *Pool) Run(ctx context.Context) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.worker.Pool.Run")
	if p == nil {
		return errors.New("agent worker pool: nil receiver")
	}
	if len(p.slots) == 0 {
		return errors.New("agent worker pool: no slots")
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(p.slots))
	for _, slot := range p.slots {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			if err := w.Run(ctx); err != nil {
				errCh <- err
			}
		}(slot)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return fmt.Errorf("agent worker pool: %w", err)
		}
	}
	return nil
}

// CancelCurrentRun cancels in-flight runner runs on every slot.
func (p *Pool) CancelCurrentRun() bool {
	if p == nil {
		return false
	}
	cancelled := false
	for _, slot := range p.slots {
		if slot != nil && slot.CancelCurrentRun() {
			cancelled = true
		}
	}
	return cancelled
}

// Slots exposes pool workers for tests.
func (p *Pool) Slots() []*Worker {
	if p == nil {
		return nil
	}
	return p.slots
}
