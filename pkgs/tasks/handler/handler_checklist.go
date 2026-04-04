package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type checklistItemCreateJSON struct {
	Text string `json:"text"`
}

type checklistItemDoneJSON struct {
	Done bool `json:"done"`
}

type checklistListResponse struct {
	Items []store.ChecklistItemView `json:"items"`
}

func (h *Handler) getChecklist(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.getChecklist")
	const op = "tasks.checklist.list"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	debugHTTPRequest(r, op, "task_id", id)
	items, err := h.store.ListChecklistForSubject(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) postChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.postChecklistItem")
	const op = "tasks.checklist.create"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	var body checklistItemCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", id, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", id, "actor", string(by),
		"text_len", len(body.Text), "text_preview", truncateRunes(body.Text, maxHTTPLogTextRunes))
	it, err := h.store.AddChecklistItem(r.Context(), id, body.Text, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	writeJSON(w, r, op, http.StatusCreated, it)
}

func (h *Handler) patchChecklistItemDone(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.patchChecklistItemDone")
	const op = "tasks.checklist.done"
	r = withCallRoot(r, op)
	taskID := strings.TrimSpace(r.PathValue("id"))
	itemID := strings.TrimSpace(r.PathValue("itemId"))
	var body checklistItemDoneJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "done", body.Done, "actor", string(by))
	if err := h.store.SetChecklistItemDone(r.Context(), taskID, itemID, body.Done, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, taskID)
	items, err := h.store.ListChecklistForSubject(r.Context(), taskID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) deleteChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.deleteChecklistItem")
	const op = "tasks.checklist.delete"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	itemID := strings.TrimSpace(r.PathValue("itemId"))
	debugHTTPRequest(r, op, "task_id", id, "item_id", itemID)
	if err := h.store.DeleteChecklistItem(r.Context(), id, itemID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "task_id", id, "item_id", itemID, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}
