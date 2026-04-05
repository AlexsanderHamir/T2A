package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.create")
	const op = "tasks.create"
	r = withCallRoot(r, op)
	var body taskCreateJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	by := actorFromRequest(r)
	debugHTTPRequest(r, op, taskCreateInputFields(&body, string(by))...)
	if h.repo != nil {
		if err := h.repo.ValidatePromptMentions(body.InitialPrompt); err != nil {
			writeJSONError(w, r, op, http.StatusBadRequest, err.Error())
			return
		}
	}
	inherit := false
	if body.ChecklistInherit != nil {
		inherit = *body.ChecklistInherit
	}
	t, err := h.store.Create(r.Context(), store.CreateTaskInput{
		ID:               body.ID,
		Title:            body.Title,
		InitialPrompt:    body.InitialPrompt,
		Status:           body.Status,
		Priority:         body.Priority,
		ParentID:         body.ParentID,
		ChecklistInherit: inherit,
	}, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	tree, err := h.store.GetTaskTree(r.Context(), t.ID)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskCreated, t.ID)
	if t.ParentID != nil && *t.ParentID != "" {
		h.notifyChange(TaskUpdated, *t.ParentID)
	}
	writeJSON(w, r, op, http.StatusCreated, tree)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.get")
	const op = "tasks.get"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	debugHTTPRequest(r, op, "task_id", id)
	t, err := h.store.GetTaskTree(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.list")
	const op = "tasks.list"
	r = withCallRoot(r, op)
	limit, offset, err := parseListParams(r.Context(), r.URL.Query())
	if err != nil {
		debugHTTPRequest(r, op, "list_params_invalid", true)
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "limit", limit, "offset", offset)
	tasks, err := h.store.ListRootForest(r.Context(), limit, offset)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, listResponse{Tasks: tasks, Limit: limit, Offset: offset})
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.patch")
	const op = "tasks.patch"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	var body taskPatchJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", id, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	debugHTTPRequest(r, op, append(append([]any{}, "task_id", id), taskPatchInputFields(&body)...)...)
	in := store.UpdateTaskInput{
		Title:            body.Title,
		InitialPrompt:    body.InitialPrompt,
		Status:           body.Status,
		Priority:         body.Priority,
		ChecklistInherit: body.ChecklistInherit,
	}
	if body.ParentID.Defined {
		if body.ParentID.Clear {
			in.Parent = &store.ParentFieldPatch{Clear: true}
		} else {
			in.Parent = &store.ParentFieldPatch{ID: body.ParentID.SetID}
		}
	}
	if h.repo != nil && body.InitialPrompt != nil {
		if err := h.repo.ValidatePromptMentions(*body.InitialPrompt); err != nil {
			writeJSONError(w, r, op, http.StatusBadRequest, err.Error())
			return
		}
	}
	by := actorFromRequest(r)
	if _, err := h.store.Update(r.Context(), id, in, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	tree, err := h.store.GetTaskTree(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	writeJSON(w, r, op, http.StatusOK, tree)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.delete")
	const op = "tasks.delete"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	debugHTTPRequest(r, op, "task_id", id)
	by := actorFromRequest(r)
	parentNotify, err := h.store.Delete(r.Context(), id, by)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskDeleted, id)
	if parentNotify != "" {
		h.notifyChange(TaskUpdated, parentNotify)
	}
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "task_id", id, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}

func parseListParams(ctx context.Context, q url.Values) (limit, offset int, err error) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.parseListParams")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = PushCall(ctx, "parseListParams")
	helperDebugIn(ctx, "parseListParams", "limit_q", q.Get("limit"), "offset_q", q.Get("offset"))
	defer func() { helperDebugOut(ctx, "parseListParams", "limit", limit, "offset", offset, "err", err) }()
	limit = 50
	offset = 0
	if v := q.Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || n > 200 {
			return 0, 0, fmt.Errorf("%w: limit must be integer 0..200", domain.ErrInvalidInput)
		}
		limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			return 0, 0, fmt.Errorf("%w: offset must be non-negative integer", domain.ErrInvalidInput)
		}
		offset = n
	}
	return limit, offset, nil
}
