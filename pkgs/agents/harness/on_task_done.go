package harness

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

type onTaskDoneCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message,omitempty"`
}

type onTaskDonePayload struct {
	WorktreeID string             `json:"worktree_id,omitempty"`
	Commits    []onTaskDoneCommit `json:"commits"`
}

func (h *Harness) emitOnTaskDone(ctx context.Context, task *domain.Task, cycleID string) {
	if h == nil || h.store == nil || task == nil {
		return
	}
	payload := onTaskDonePayload{Commits: []onTaskDoneCommit{}}
	if task.WorktreeID != nil {
		payload.WorktreeID = *task.WorktreeID
	}
	rows, err := h.store.ListCommitsForCycle(ctx, cycleID)
	if err != nil {
		slog.Warn("on_task_done: list cycle commits failed",
			"cmd", calltrace.LogCmd, "operation", "agent.harness.Harness.emitOnTaskDone",
			"task_id", task.ID, "cycle_id", cycleID, "err", err)
	} else {
		for _, c := range rows {
			payload.Commits = append(payload.Commits, onTaskDoneCommit{
				SHA:     c.SHA,
				Message: c.Message,
			})
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("on_task_done: marshal payload failed",
			"cmd", calltrace.LogCmd, "operation", "agent.harness.Harness.emitOnTaskDone",
			"task_id", task.ID, "err", err)
		return
	}
	if err := h.store.AppendTaskEvent(ctx, task.ID, domain.EventOnTaskDone, domain.ActorAgent, raw); err != nil {
		slog.Warn("on_task_done: append event failed",
			"cmd", calltrace.LogCmd, "operation", "agent.harness.Harness.emitOnTaskDone",
			"task_id", task.ID, "err", err)
	}
}
