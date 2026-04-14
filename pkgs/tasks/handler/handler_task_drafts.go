package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
)

func (h *Handler) listTaskDrafts(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.listTaskDrafts")
	const op = "task_drafts.list"
	r = calltrace.WithRequestRoot(r, op)
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if len(raw) > maxListIntQueryParamBytes {
			writeJSONError(w, r, op, http.StatusBadRequest, "limit value too long")
			return
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 100 {
			writeJSONError(w, r, op, http.StatusBadRequest, "limit must be integer 0..100")
			return
		}
		limit = n
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := h.store.ListDrafts(r.Context(), limit)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, map[string]any{"drafts": rows})
}

func (h *Handler) saveTaskDraft(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.saveTaskDraft")
	const op = "task_drafts.save"
	r = calltrace.WithRequestRoot(r, op)
	var body taskDraftSaveJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	saved, err := h.store.SaveDraft(r.Context(), body.ID, body.Name, body.Payload)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, saved)
}

func (h *Handler) getTaskDraft(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getTaskDraft")
	const op = "task_drafts.get"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	row, err := h.store.GetDraft(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, row)
}

func (h *Handler) deleteTaskDraft(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteTaskDraft")
	const op = "task_drafts.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	if err := h.store.DeleteDraft(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "draft_id", id, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}
