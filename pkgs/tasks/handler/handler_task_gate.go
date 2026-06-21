package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func (h *Handler) patchTaskGate(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchTaskGate")
	const op = "tasks.gate.patch"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body taskGateActionJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	action := strings.TrimSpace(strings.ToLower(body.Action))
	if action == "" {
		writeStoreError(w, r, op, fmt.Errorf("%w: action required", domain.ErrInvalidInput))
		return
	}
	by := actorFromRequest(r)
	if _, err := h.store.ApplyTaskGateAction(r.Context(), id, action, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	task, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskGateChanged, id)
	h.notifyTaskChanged(TaskUpdated, id, task)
	writeJSON(w, r, op, http.StatusOK, task)
}
