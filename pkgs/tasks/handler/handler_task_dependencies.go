package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func (h *Handler) listTaskDependencies(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listTaskDependencies")
	const op = "tasks.dependencies.list"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if _, err := h.store.Get(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	deps, err := h.store.ListTaskDependencies(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if deps == nil {
		deps = []string{}
	}
	writeJSON(w, r, op, http.StatusOK, taskDependenciesListResponse{DependsOn: deps})
}

func (h *Handler) addTaskDependency(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.addTaskDependency")
	const op = "tasks.dependencies.create"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body taskDependencyCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	depID := strings.TrimSpace(body.DependsOnTaskID)
	if depID == "" {
		writeStoreError(w, r, op, fmt.Errorf("%w: depends_on_task_id required", domain.ErrInvalidInput))
		return
	}
	if err := h.store.AddTaskDependency(r.Context(), id, depID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskDependencyChanged, id)
	deps, err := h.store.ListTaskDependencies(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if deps == nil {
		deps = []string{}
	}
	writeJSON(w, r, op, http.StatusCreated, taskDependenciesListResponse{DependsOn: deps})
}

func (h *Handler) removeTaskDependency(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.removeTaskDependency")
	const op = "tasks.dependencies.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	depID, err := parseTaskPathID(r.PathValue("depId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.RemoveTaskDependency(r.Context(), id, depID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskDependencyChanged, id)
	w.WriteHeader(http.StatusNoContent)
}
