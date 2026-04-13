package agents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

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

// ReconcileReadyUserTasksNotQueued loads ready user-created tasks from the store and enqueues
// any whose ids are not already pending in q. Pagination uses store.ListReadyTasksUserCreated with
// after_id until a short page or an empty page.
func ReconcileReadyUserTasksNotQueued(ctx context.Context, st *store.Store, q *MemoryQueue, pageSize int) (ReconcileResult, error) {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.ReconcileReadyUserTasksNotQueued")
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
	afterID := ""
	for {
		batch, err := st.ListReadyTasksUserCreated(ctx, pageSize, afterID)
		if err != nil {
			return res, fmt.Errorf("agents reconcile: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, t := range batch {
			res.Scanned++
			err := q.NotifyUserTaskCreated(ctx, t)
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
		afterID = batch[len(batch)-1].ID
	}
	return res, nil
}

// RunReconcileLoop invokes ReconcileReadyUserTasksNotQueued once immediately, then every tickInterval
// while ctx is active. When tickInterval <= 0, only the initial run executes.
func RunReconcileLoop(ctx context.Context, st *store.Store, q *MemoryQueue, tickInterval time.Duration) {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.RunReconcileLoop", "tick_interval", tickInterval.String())
	runOnce := func() {
		res, err := ReconcileReadyUserTasksNotQueued(ctx, st, q, 200)
		if err != nil {
			slog.Warn("user task agent reconcile failed", "cmd", agentsLogCmd, "operation", "agents.reconcile_once", "err", err)
			return
		}
		slog.Info("user task agent reconcile done", "cmd", agentsLogCmd, "operation", "agents.reconcile_once",
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
