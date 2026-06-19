package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

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

func (w *Worker) deferTaskPickup(ctx context.Context, taskID string, delay time.Duration) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.deferTaskPickup",
		"task_id", taskID, "delay", delay.String())
	at := w.clock().Add(delay).UTC()
	patch := store.PickupNotBeforePatch{At: at}
	if _, err := w.store.Update(ctx, taskID, store.UpdateTaskInput{PickupNotBefore: &patch}, domain.ActorAgent); err != nil {
		slog.Warn("agent worker defer pickup failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.deferTaskPickup.err", "task_id", taskID, "err", err)
	}
}

// transitionTaskToRunning flips the task to running before the harness runs.
// Returns the post-pickup task row and any consumed retry intent.
func (w *Worker) transitionTaskToRunning(ctx context.Context, taskID string) (*domain.Task, *domain.PendingRetry, bool) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.transitionTaskToRunning",
		"task_id", taskID)
	res, err := w.store.AgentPickup(ctx, taskID, domain.ActorAgent)
	if err != nil {
		level := slog.LevelWarn
		if errors.Is(err, domain.ErrNotFound) {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "agent worker task pickup failed",
			"cmd", workerLogCmd, "operation", "agent.worker.Worker.transitionTaskToRunning.err",
			"task_id", taskID, "err", err)
		return nil, nil, false
	}
	return res.Task, res.ConsumedRetry, true
}

func (w *Worker) openRunningCycle(ctx context.Context, taskID string) (*domain.TaskCycle, bool) {
	cycles, err := w.store.ListCyclesForTask(ctx, taskID, 0)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, false
		}
		slog.Warn("agent worker list cycles failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.openRunningCycle.err", "task_id", taskID, "err", err)
		return nil, false
	}
	for i := len(cycles) - 1; i >= 0; i-- {
		if cycles[i].Status == domain.CycleStatusRunning {
			cycle := cycles[i]
			return &cycle, true
		}
	}
	return nil, false
}

// processOne runs queue admission then delegates the cycle body to the harness.
func (w *Worker) processOne(parentCtx context.Context, task domain.Task) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.processOne",
		"task_id", task.ID)
	defer w.queue.AckAfterRecv(task.ID)
	defer w.recoverAdmissionPanic(task.ID)

	fresh, ok := w.reloadTask(parentCtx, task.ID)
	if !ok {
		return
	}

	switch fresh.Status {
	case domain.StatusRunning:
		cycle, ok := w.openRunningCycle(parentCtx, fresh.ID)
		if !ok {
			slog.Warn("running task without open cycle at dequeue", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.processOne.no_open_cycle", "task_id", task.ID)
			return
		}
		w.harness.Resume(parentCtx, fresh, cycle)
		return
	case domain.StatusReady:
		// continue below
	default:
		slog.Warn("stale task at dequeue", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.processOne.stale", "task_id", task.ID,
			"status", string(fresh.Status))
		return
	}

	now := w.clock()
	ready, err := w.store.ReadyForAgentPickup(parentCtx, fresh, now)
	if err != nil {
		slog.Warn("agent worker readiness check failed", "cmd", workerLogCmd,
			"operation", "agent.worker.Worker.processOne.readiness", "task_id", task.ID, "err", err)
		return
	}
	if !ready {
		w.deferTaskPickup(parentCtx, task.ID, 60*time.Second)
		return
	}
	picked, consumedRetry, ok := w.transitionTaskToRunning(parentCtx, task.ID)
	if !ok {
		return
	}
	w.harness.RunWithRetry(parentCtx, picked, consumedRetry)
}

func (w *Worker) recoverAdmissionPanic(taskID string) {
	if recover() == nil {
		return
	}
	slog.Error("agent worker admission panic", "cmd", workerLogCmd,
		"operation", "agent.worker.Worker.recoverAdmissionPanic", "task_id", taskID)
	bg, cancel := context.WithTimeout(context.Background(), DefaultShutdownAbortTimeout)
	defer cancel()
	failed := domain.StatusFailed
	if _, err := w.store.Update(bg, taskID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("agent worker admission panic task transition failed", "cmd", workerLogCmd,
				"operation", "agent.worker.Worker.recoverAdmissionPanic.err",
				"task_id", taskID, "err", err)
		}
	}
}

func (w *Worker) clock() time.Time {
	if w.opts.Clock != nil {
		return w.opts.Clock()
	}
	return time.Now().UTC()
}
