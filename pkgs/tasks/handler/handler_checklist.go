package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type checklistItemCreateJSON struct {
	Text string `json:"text"`
}

type patchChecklistItemBody struct {
	Text *string `json:"text,omitempty"`
	Done *bool   `json:"done,omitempty"`
}

type checklistListResponse struct {
	Items []store.ChecklistItemView `json:"items"`
}

func (h *Handler) getChecklist(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.getChecklist")
	const op = "tasks.checklist.list"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id)
	items, err := h.store.ListChecklistForSubject(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) postChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.postChecklistItem")
	const op = "tasks.checklist.create"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
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

func (h *Handler) patchChecklistItem(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchChecklistItem")
	const op = "tasks.checklist.patch"
	r = calltrace.WithRequestRoot(r, op)
	taskID, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	itemID, err := parseTaskPathItemID(r.PathValue("itemId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var body patchChecklistItemBody
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	textSet := body.Text != nil
	doneSet := body.Done != nil
	if textSet == doneSet {
		writeStoreError(w, r, op, fmt.Errorf("%w: send exactly one of text or done", domain.ErrInvalidInput))
		return
	}
	by := actorFromRequest(r)
	if textSet {
		t := strings.TrimSpace(*body.Text)
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "text_len", len(t), "text_preview", truncateRunes(t, maxHTTPLogTextRunes), "actor", string(by))
		if t == "" {
			writeStoreError(w, r, op, fmt.Errorf("%w: text required", domain.ErrInvalidInput))
			return
		}
		if err := h.store.UpdateChecklistItemText(r.Context(), taskID, itemID, t, by); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
	} else {
		debugHTTPRequest(r, op, "task_id", taskID, "item_id", itemID, "done", *body.Done, "actor", string(by))
		if err := h.store.SetChecklistItemDone(r.Context(), taskID, itemID, *body.Done, by); err != nil {
			writeStoreError(w, r, op, err)
			return
		}
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.deleteChecklistItem")
	const op = "tasks.checklist.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	itemID, err := parseTaskPathItemID(r.PathValue("itemId"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id, "item_id", itemID)
	by := actorFromRequest(r)
	if err := h.store.DeleteChecklistItem(r.Context(), id, itemID, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "task_id", id, "item_id", itemID, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}
