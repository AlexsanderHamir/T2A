package agents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// ReconcileTickInterval is the fixed period between background
// ReconcileReadyTasksNotQueued passes after startup. It is not
// configurable; the pickup wake scheduler provides low-latency deferred
// pickup while this tick remains a durable backstop.
const ReconcileTickInterval = 2 * time.Minute

// ReconcileResult summarizes one reconcile pass.
type ReconcileResult struct {
	Scanned int
	// Enqueued counts tasks newly written to the queue (Notify returned nil).
	Enqueued int
	// SkippedAlreadyQueued counts tasks that were already pending in the queue.
	SkippedAlreadyQueued int
	// StoppedOnQueueFull is true when a page stopped early because the buffer was full.
	StoppedOnQueueFull bool
}

// ReconcileReadyTasksNotQueued loads ready tasks from the store and enqueues any whose ids are
// not already pending in q. Pagination uses store.ListReadyTaskQueueCandidates (FIFO by
// task_created time, then id) so older backlog is offered slots before lexicographic id order alone would.
func ReconcileReadyTasksNotQueued(ctx context.Context, st *store.Store, q *MemoryQueue, pageSize int) (ReconcileResult, error) {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.ReconcileReadyTasksNotQueued")
	var res ReconcileResult
	if st == nil {
		return res, errors.New("agents: nil store")
	}
	if q == nil {
		return res, errors.New("agents: nil MemoryQueue")
	}
	if pageSize <= 0 {
		pageSize = 200
	}
	var pageCursor *store.ReadyTaskQueueCursor
	for {
		batch, err := st.ListReadyTaskQueueCandidates(ctx, pageSize, pageCursor)
		if err != nil {
			return res, fmt.Errorf("agents reconcile: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, row := range batch {
			res.Scanned++
			err := q.NotifyReadyTask(ctx, row.Task)
			switch {
			case err == nil:
				res.Enqueued++
			case errors.Is(err, ErrAlreadyQueued):
				res.SkippedAlreadyQueued++
			case errors.Is(err, ErrQueueFull):
				res.StoppedOnQueueFull = true
				return res, nil
			default:
				return res, err
			}
		}
		if len(batch) < pageSize {
			break
		}
		last := batch[len(batch)-1]
		pageCursor = &store.ReadyTaskQueueCursor{
			AfterTaskCreatedAt: last.TaskCreatedAt,
			AfterTaskID:        last.Task.ID,
			AfterEventRowID:    last.EventRowID,
		}
	}
	return res, nil
}

// RunReconcileLoop invokes ReconcileReadyTasksNotQueued once immediately, then every tickInterval
// while ctx is active. When tickInterval <= 0, only the initial run executes.
func RunReconcileLoop(ctx context.Context, st *store.Store, q *MemoryQueue, tickInterval time.Duration) {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.RunReconcileLoop", "tick_interval", tickInterval.String())
	runOnce := func() {
		res, err := ReconcileReadyTasksNotQueued(ctx, st, q, 200)
		if err != nil {
			slog.Warn("ready task agent reconcile failed", "cmd", agentsLogCmd, "operation", "agents.reconcile_once", "err", err)
			return
		}
		slog.Info("ready task agent reconcile done", "cmd", agentsLogCmd, "operation", "agents.reconcile_once",
			"scanned", res.Scanned, "enqueued", res.Enqueued, "skipped_already_queued", res.SkippedAlreadyQueued,
			"stopped_on_queue_full", res.StoppedOnQueueFull)
	}
	runOnce()
	if tickInterval <= 0 {
		return
	}
	t := time.NewTicker(tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runOnce()
		}
	}
}
