package handler

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (h *Handler) evaluateDraft(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.evaluateDraft")
	const op = "tasks.evaluate"
	r = withCallRoot(r, op)
	var body taskEvaluateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	debugHTTPRequest(r, op,
		"draft_id", body.ID,
		"title_len", len(body.Title),
		"initial_prompt_len", len(body.InitialPrompt),
		"checklist_items_count", len(body.ChecklistItems),
	)
	if h.repo != nil {
		if err := h.repo.ValidatePromptMentions(body.InitialPrompt); err != nil {
			writeJSONError(w, r, op, http.StatusBadRequest, err.Error())
			return
		}
	}
	out, err := h.store.EvaluateDraftTask(r.Context(), store.EvaluateDraftTaskInput{
		DraftID:          body.ID,
		Title:            body.Title,
		InitialPrompt:    body.InitialPrompt,
		Status:           body.Status,
		Priority:         body.Priority,
		TaskType:         body.TaskType,
		ParentID:         body.ParentID,
		ChecklistInherit: body.ChecklistInherit,
		ChecklistItems:   body.ChecklistItems,
	}, actorFromRequest(r))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusCreated, out)
}
