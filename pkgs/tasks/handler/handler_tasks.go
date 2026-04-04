package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskCreateJSON struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	InitialPrompt    string          `json:"initial_prompt"`
	Status           domain.Status   `json:"status"`
	Priority         domain.Priority `json:"priority"`
	ParentID         *string         `json:"parent_id"`
	ChecklistInherit *bool           `json:"checklist_inherit"`
}

type taskPatchJSON struct {
	Title            *string          `json:"title"`
	InitialPrompt    *string          `json:"initial_prompt"`
	Status           *domain.Status   `json:"status"`
	Priority         *domain.Priority `json:"priority"`
	ParentID         patchParentField `json:"parent_id"`
	ChecklistInherit *bool            `json:"checklist_inherit"`
}

type listResponse struct {
	Tasks  []store.TaskNode `json:"tasks"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type taskEventLine struct {
	Seq            int64                        `json:"seq"`
	At             time.Time                    `json:"at"`
	Type           domain.EventType             `json:"type"`
	By             domain.Actor                 `json:"by"`
	Data           json.RawMessage              `json:"data"`
	UserResponse   *string                      `json:"user_response,omitempty"`
	UserResponseAt *time.Time                   `json:"user_response_at,omitempty"`
	ResponseThread []domain.ResponseThreadEntry `json:"response_thread,omitempty"`
}

type taskEventsResponse struct {
	TaskID          string          `json:"task_id"`
	Events          []taskEventLine `json:"events"`
	Limit           *int            `json:"limit,omitempty"`
	Total           *int64          `json:"total,omitempty"`
	RangeStart      *int64          `json:"range_start,omitempty"`
	RangeEnd        *int64          `json:"range_end,omitempty"`
	HasMoreNewer    bool            `json:"has_more_newer"`
	HasMoreOlder    bool            `json:"has_more_older"`
	ApprovalPending bool            `json:"approval_pending"`
}

type taskEventDetailResponse struct {
	TaskID         string                       `json:"task_id"`
	Seq            int64                        `json:"seq"`
	At             time.Time                    `json:"at"`
	Type           domain.EventType             `json:"type"`
	By             domain.Actor                 `json:"by"`
	Data           json.RawMessage              `json:"data"`
	UserResponse   *string                      `json:"user_response,omitempty"`
	UserResponseAt *time.Time                   `json:"user_response_at,omitempty"`
	ResponseThread []domain.ResponseThreadEntry `json:"response_thread,omitempty"`
}

type taskEventUserResponseJSON struct {
	UserResponse string `json:"user_response"`
}

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
	writeJSON(w, r, op, http.StatusCreated, tree)
}

func (h *Handler) taskEvent(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.taskEvent")
	const op = "tasks.event"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	seqStr := strings.TrimSpace(r.PathValue("seq"))
	seq, err := strconv.ParseInt(seqStr, 10, 64)
	if err != nil || seq < 1 {
		debugHTTPRequest(r, op, "task_id", id, "seq_param", seqStr, "seq_parse_failed", true)
		writeError(w, r, op, errors.New("seq must be a positive integer"), http.StatusBadRequest)
		return
	}
	debugHTTPRequest(r, op, "task_id", id, "seq", seq)
	ev, err := h.store.GetTaskEvent(r.Context(), id, seq)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	writeJSON(w, r, op, http.StatusOK, taskEventDetailFromDomain(ev, id))
}

func taskEventDetailFromDomain(ev *domain.TaskEvent, taskID string) taskEventDetailResponse {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.taskEventDetailFromDomain")
	data := json.RawMessage(ev.Data)
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	resp := taskEventDetailResponse{
		TaskID:         taskID,
		Seq:            ev.Seq,
		At:             ev.At,
		Type:           ev.Type,
		By:             ev.By,
		Data:           data,
		UserResponse:   ev.UserResponse,
		UserResponseAt: ev.UserResponseAt,
	}
	if th := store.ThreadEntriesForDisplay(ev); len(th) > 0 {
		resp.ResponseThread = th
	}
	return resp
}

func taskEventLines(evs []domain.TaskEvent) []taskEventLine {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.taskEventLines")
	out := make([]taskEventLine, 0, len(evs))
	for _, e := range evs {
		data := json.RawMessage(e.Data)
		if len(data) == 0 {
			data = json.RawMessage(`{}`)
		}
		line := taskEventLine{
			Seq:            e.Seq,
			At:             e.At,
			Type:           e.Type,
			By:             e.By,
			Data:           data,
			UserResponse:   e.UserResponse,
			UserResponseAt: e.UserResponseAt,
		}
		if th := store.ThreadEntriesForDisplay(&e); len(th) > 0 {
			line.ResponseThread = th
		}
		out = append(out, line)
	}
	return out
}

func (h *Handler) taskEvents(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.taskEvents")
	const op = "tasks.events"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	debugHTTPRequest(r, op, "task_id", id)
	if _, err := h.store.Get(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	pending, err := h.store.ApprovalPending(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	q := r.URL.Query()
	if q.Get("offset") != "" {
		writeStoreError(w, r, op, fmt.Errorf("%w: offset is not supported for task events; use before_seq or after_seq", domain.ErrInvalidInput))
		return
	}
	if q.Get("limit") == "" && q.Get("before_seq") == "" && q.Get("after_seq") == "" {
		evs, err := h.store.ListTaskEvents(r.Context(), id)
		if err != nil {
			writeStoreError(w, r, op, err)
			return
		}
		writeJSON(w, r, op, http.StatusOK, taskEventsResponse{
			TaskID:          id,
			Events:          taskEventLines(evs),
			ApprovalPending: pending,
		})
		return
	}
	beforeStr := strings.TrimSpace(q.Get("before_seq"))
	afterStr := strings.TrimSpace(q.Get("after_seq"))
	if beforeStr != "" && afterStr != "" {
		writeError(w, r, op, errors.New("before_seq and after_seq cannot both be set"), http.StatusBadRequest)
		return
	}
	limit, err := parseTaskEventsLimit(r.Context(), q)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	var beforeSeq, afterSeq *int64
	if beforeStr != "" {
		n, err := strconv.ParseInt(beforeStr, 10, 64)
		if err != nil || n < 1 {
			writeError(w, r, op, errors.New("before_seq must be a positive integer"), http.StatusBadRequest)
			return
		}
		beforeSeq = &n
	}
	if afterStr != "" {
		n, err := strconv.ParseInt(afterStr, 10, 64)
		if err != nil || n < 1 {
			writeError(w, r, op, errors.New("after_seq must be a positive integer"), http.StatusBadRequest)
			return
		}
		afterSeq = &n
	}
	page, err := h.store.ListTaskEventsPageCursor(r.Context(), id, limit, beforeSeq, afterSeq)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	lim := limit
	tot := page.Total
	resp := taskEventsResponse{
		TaskID:          id,
		Events:          taskEventLines(page.Events),
		Limit:           &lim,
		Total:           &tot,
		HasMoreNewer:    page.HasMoreNewer,
		HasMoreOlder:    page.HasMoreOlder,
		ApprovalPending: pending,
	}
	if len(page.Events) > 0 {
		rs := page.RangeStart
		re := page.RangeEnd
		resp.RangeStart = &rs
		resp.RangeEnd = &re
	}
	writeJSON(w, r, op, http.StatusOK, resp)
}

func (h *Handler) patchTaskEventUserResponse(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.patchTaskEventUserResponse")
	const op = "tasks.event.user_response"
	r = withCallRoot(r, op)
	id := strings.TrimSpace(r.PathValue("id"))
	seqStr := strings.TrimSpace(r.PathValue("seq"))
	seq, err := strconv.ParseInt(seqStr, 10, 64)
	if err != nil || seq < 1 {
		debugHTTPRequest(r, op, "task_id", id, "seq_param", seqStr, "seq_parse_failed", true)
		writeError(w, r, op, errors.New("seq must be a positive integer"), http.StatusBadRequest)
		return
	}
	var body taskEventUserResponseJSON
	if err := decodeJSON(r.Context(), r.Body, &body); err != nil {
		debugHTTPRequest(r, op, "task_id", id, "seq", seq, "json_decode_failed", true)
		writeError(w, r, op, err, http.StatusBadRequest)
		return
	}
	debugHTTPRequest(r, op, "task_id", id, "seq", seq,
		"user_response_len", len(body.UserResponse),
		"user_response_preview", truncateRunes(body.UserResponse, maxHTTPLogTextRunes),
	)
	by := actorFromRequest(r)
	if err := h.store.AppendTaskEventResponseMessage(r.Context(), id, seq, body.UserResponse, by); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	ev, err := h.store.GetTaskEvent(r.Context(), id, seq)
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskUpdated, id)
	writeJSON(w, r, op, http.StatusOK, taskEventDetailFromDomain(ev, id))
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
	if err := h.store.Delete(r.Context(), id); err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	h.notifyChange(TaskDeleted, id)
	debugHTTPOut(r.Context(), op, http.StatusNoContent, "task_id", id, "response_empty", true)
	w.WriteHeader(http.StatusNoContent)
}

func parseTaskEventsLimit(ctx context.Context, q url.Values) (limit int, err error) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.parseTaskEventsLimit")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = PushCall(ctx, "parseTaskEventsLimit")
	helperDebugIn(ctx, "parseTaskEventsLimit", "limit_q", q.Get("limit"), "before_seq_q", q.Get("before_seq"), "after_seq_q", q.Get("after_seq"))
	defer func() { helperDebugOut(ctx, "parseTaskEventsLimit", "limit", limit, "err", err) }()
	limit = 50
	if v := q.Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || n > 200 {
			return 0, fmt.Errorf("%w: limit must be integer 0..200", domain.ErrInvalidInput)
		}
		limit = n
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return limit, nil
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
