package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

const (
	maxListIntQueryParamBytes = 32
	maxListAfterIDParamBytes  = 128
)

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.create")
	const op = "tasks.create"
	r = calltrace.WithRequestRoot(r, op)
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
		DraftID:          body.DraftID,
		Title:            body.Title,
		InitialPrompt:    body.InitialPrompt,
		Status:           body.Status,
		Priority:         body.Priority,
		TaskType:         body.TaskType,
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
	taskapiDomainTasksCreatedTotal.Inc()
	writeJSON(w, r, op, http.StatusCreated, tree)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.get")
	const op = "tasks.get"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "task_id", id)
	t, err := h.store.GetTaskTree(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.list")
	const op = "tasks.list"
	r = calltrace.WithRequestRoot(r, op)
	limit, offset, afterID, err := parseListParams(r.Context(), r.URL.Query())
	if err != nil {
		debugHTTPRequest(r, op, "list_params_invalid", true)
		writeStoreError(w, r, op, err)
		return
	}
	debugHTTPRequest(r, op, "limit", limit, "offset", offset, "after_id", afterID)
	var tasks []store.TaskNode
	var hasMore bool
	if afterID != "" {
		tasks, hasMore, err = h.store.ListRootForestAfter(r.Context(), limit, afterID)
		offset = 0
	} else {
		tasks, hasMore, err = h.store.ListRootForest(r.Context(), limit, offset)
	}
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, listResponse{Tasks: tasks, Limit: limit, Offset: offset, HasMore: hasMore})
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.stats")
	const op = "tasks.stats"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op)
	stats, err := h.store.TaskStats(r.Context())
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, taskStatsResponse{
		Total:      stats.Total,
		Ready:      stats.Ready,
		Critical:   stats.Critical,
		ByStatus:   stats.ByStatus,
		ByPriority: stats.ByPriority,
		ByScope:    stats.ByScope,
	})
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patch")
	const op = "tasks.patch"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
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
		TaskType:         body.TaskType,
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
	taskapiDomainTasksUpdatedTotal.Inc()
	writeJSON(w, r, op, http.StatusOK, tree)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.delete")
	const op = "tasks.delete"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
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
	taskapiDomainTasksDeletedTotal.Inc()
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "task_id", id, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}

func parseListParams(ctx context.Context, q url.Values) (limit, offset int, afterID string, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseListParams")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = calltrace.Push(ctx, "parseListParams")
	calltrace.HelperIOIn(ctx, "parseListParams", "limit_q", q.Get("limit"), "offset_q", q.Get("offset"), "after_id_q", q.Get("after_id"))
	defer func() {
		calltrace.HelperIOOut(ctx, "parseListParams", "limit", limit, "offset", offset, "after_id", afterID, "err", err)
	}()
	limit = 50
	offset = 0
	afterID = strings.TrimSpace(q.Get("after_id"))
	if afterID != "" && len(afterID) > maxListAfterIDParamBytes {
		return 0, 0, "", fmt.Errorf("%w: after_id too long", domain.ErrInvalidInput)
	}
	if _, ok := q["offset"]; ok && afterID != "" {
		return 0, 0, "", fmt.Errorf("%w: offset cannot be used with after_id", domain.ErrInvalidInput)
	}
	if afterID != "" {
		if _, perr := uuid.Parse(afterID); perr != nil {
			return 0, 0, "", fmt.Errorf("%w: after_id must be a UUID", domain.ErrInvalidInput)
		}
	}
	if v := q.Get("limit"); v != "" {
		if len(v) > maxListIntQueryParamBytes {
			return 0, 0, "", fmt.Errorf("%w: limit value too long", domain.ErrInvalidInput)
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || n > 200 {
			return 0, 0, "", fmt.Errorf("%w: limit must be integer 0..200", domain.ErrInvalidInput)
		}
		limit = n
	}
	if limit <= 0 {
		limit = 50
	}
	if v := q.Get("offset"); v != "" {
		if len(v) > maxListIntQueryParamBytes {
			return 0, 0, "", fmt.Errorf("%w: offset value too long", domain.ErrInvalidInput)
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			return 0, 0, "", fmt.Errorf("%w: offset must be non-negative integer", domain.ErrInvalidInput)
		}
		offset = n
	}
	return limit, offset, afterID, nil
}
