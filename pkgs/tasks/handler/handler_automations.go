package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (h *Handler) createAutomation(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.createAutomation")
	const op = "automations.create"
	r = calltrace.WithRequestRoot(r, op)
	var body automationCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	row, err := h.store.CreateAutomation(r.Context(), store.CreateAutomationInput{
		ID:          body.ID,
		Title:       body.Title,
		Description: body.Description,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, row)
}

func (h *Handler) listAutomations(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listAutomations")
	const op = "automations.list"
	r = calltrace.WithRequestRoot(r, op)
	limit, includeArchived, err := parseAutomationListParams(r.URL.Query())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	rows, err := h.store.ListAutomations(r.Context(), includeArchived, limit)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, automationsListResponse{Automations: rows, Limit: limit})
}

func (h *Handler) getAutomation(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getAutomation")
	const op = "automations.get"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	row, err := h.store.GetAutomation(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSONWithETag(w, r, op, http.StatusOK, row)
}

func (h *Handler) patchAutomation(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchAutomation")
	const op = "automations.patch"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body automationPatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	if body.isEmpty() {
		writeStoreError(w, r, op, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput))
		return
	}
	row, err := h.store.UpdateAutomation(r.Context(), id, store.UpdateAutomationInput{
		Title:       body.Title,
		Description: body.Description,
	})
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, row)
}

func (h *Handler) deleteAutomation(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteAutomation")
	const op = "automations.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.ArchiveAutomation(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseAutomationListParams(q map[string][]string) (limit int, includeArchived bool, err error) {
	limit = 100
	if v := q["limit"]; len(v) > 0 && v[0] != "" {
		n, parseErr := strconv.Atoi(v[0])
		if parseErr != nil || n <= 0 {
			return 0, false, fmt.Errorf("%w: invalid limit", domain.ErrInvalidInput)
		}
		limit = n
	}
	includeArchived = len(q["include_archived"]) > 0 && q["include_archived"][0] == "true"
	return limit, includeArchived, nil
}
