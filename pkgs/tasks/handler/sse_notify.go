package handler

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

func (h *Handler) notifyChange(typ TaskChangeType, id string) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyChange", "change_type", typ)
	if h.hub == nil || id == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: typ, ID: id})
}

func (h *Handler) notifyTaskChanged(typ TaskChangeType, id string, data any) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyTaskChanged", "change_type", typ)
	if h.hub == nil || id == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: typ, ID: id, Data: data})
}

func (h *Handler) notifyCycleChange(taskID, cycleID string) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyCycleChange", "task_id", taskID, "cycle_id", cycleID)
	if h.hub == nil || taskID == "" || cycleID == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: TaskCycleChanged, ID: taskID, CycleID: cycleID})
}

func (h *Handler) notifyCycleChanged(taskID, cycleID string, data any) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyCycleChanged", "task_id", taskID, "cycle_id", cycleID)
	if h.hub == nil || taskID == "" || cycleID == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: TaskCycleChanged, ID: taskID, CycleID: cycleID, Data: data})
}

func (h *Handler) notifyCycleChangedFromStore(ctx context.Context, taskID, cycleID string) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyCycleChangedFromStore", "task_id", taskID, "cycle_id", cycleID)
	if h.hub == nil || taskID == "" || cycleID == "" {
		return
	}
	cycle, err := h.store.GetCycle(ctx, cycleID)
	if err != nil {
		h.notifyCycleChange(taskID, cycleID)
		return
	}
	phases, err := h.store.ListPhasesForCycle(ctx, cycleID)
	if err != nil {
		h.notifyCycleChange(taskID, cycleID)
		return
	}
	h.notifyCycleChanged(taskID, cycleID, taskCycleDetailFromDomain(cycle, phases))
}
