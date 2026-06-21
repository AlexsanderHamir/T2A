package handler

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type taskRetryJSON struct {
	Mode          domain.RetryMode `json:"mode"`
	ParentCycleID string           `json:"parent_cycle_id,omitempty"`
}

// postTaskRetry handles POST /tasks/{id}/retry for operator retry after failure.
func (h *Handler) postTaskRetry(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.postTaskRetry")
	const op = "tasks.retry"
	r = calltrace.WithRequestRoot(r, op)
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	by := actorFromRequest(r)
	if by != domain.ActorUser {
		writeStoreError(w, r, op, domain.ErrInvalidInput)
		return
	}
	var body taskRetryJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	debugHTTPRequest(r, op, "task_id", taskID, "mode", string(body.Mode), "parent_cycle_id", body.ParentCycleID)
	t, err := h.store.RequestTaskRetry(r.Context(), store.RequestRetryInput{
		TaskID:        taskID,
		Mode:          body.Mode,
		ParentCycleID: body.ParentCycleID,
	}, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyTaskChanged(TaskUpdated, taskID, t)
	taskapiDomainTasksUpdatedTotal.Inc()
	writeJSON(w, r, op, http.StatusOK, t)
}
