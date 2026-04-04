package handler

import (
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
	const op = "tasks.checklist.list"
	id := strings.TrimSpace(r.PathValue("id"))
	items, err := h.store.ListChecklistForSubject(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) postChecklistItem(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.checklist.create"
	id := strings.TrimSpace(r.PathValue("id"))
	var body checklistItemCreateJSON
	if err := decodeJSON(r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	it, err := h.store.AddChecklistItem(r.Context(), id, body.Text, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	writeJSON(w, op, http.StatusCreated, it)
}

func (h *Handler) patchChecklistItemDone(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.checklist.done"
	taskID := strings.TrimSpace(r.PathValue("id"))
	itemID := strings.TrimSpace(r.PathValue("itemId"))
	var body checklistItemDoneJSON
	if err := decodeJSON(r.Body, &body); err != nil {
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
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
	writeJSON(w, op, http.StatusOK, checklistListResponse{Items: items})
}

func (h *Handler) deleteChecklistItem(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.checklist.delete"
	id := strings.TrimSpace(r.PathValue("id"))
	itemID := strings.TrimSpace(r.PathValue("itemId"))
	if err := h.store.DeleteChecklistItem(r.Context(), id, itemID); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	w.WriteHeader(http.StatusNoContent)
}
