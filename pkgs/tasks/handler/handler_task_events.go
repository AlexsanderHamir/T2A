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

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// Seq values are small positive integers; huge path/query strings waste CPU in strconv and log fields.
const maxTaskEventSeqParamBytes = 32

func (h *Handler) taskEvent(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.taskEvent")
	const op = "tasks.event"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	seqStr := strings.TrimSpace(r.PathValue("seq"))
	if len(seqStr) > maxTaskEventSeqParamBytes {
		writeError(w, r, op, errors.New("seq too long"), http.StatusBadRequest)
		return
	}
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskEventDetailFromDomain")
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskEventLines")
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.taskEvents")
	const op = "tasks.events"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
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
	if (beforeStr != "" && len(beforeStr) > maxTaskEventSeqParamBytes) || (afterStr != "" && len(afterStr) > maxTaskEventSeqParamBytes) {
		writeError(w, r, op, errors.New("before_seq or after_seq too long"), http.StatusBadRequest)
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.patchTaskEventUserResponse")
	const op = "tasks.event.user_response"
	r = calltrace.WithRequestRoot(r, op)
	id, err := parseTaskPathID(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, r, op, err)
		return
	}
	seqStr := strings.TrimSpace(r.PathValue("seq"))
	if len(seqStr) > maxTaskEventSeqParamBytes {
		writeError(w, r, op, errors.New("seq too long"), http.StatusBadRequest)
		return
	}
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

func parseTaskEventsLimit(ctx context.Context, q url.Values) (limit int, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.parseTaskEventsLimit")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = calltrace.Push(ctx, "parseTaskEventsLimit")
	calltrace.HelperIOIn(ctx, "parseTaskEventsLimit", "limit_q", q.Get("limit"), "before_seq_q", q.Get("before_seq"), "after_seq_q", q.Get("after_seq"))
	defer func() { calltrace.HelperIOOut(ctx, "parseTaskEventsLimit", "limit", limit, "err", err) }()
	limit = 50
	if v := q.Get("limit"); v != "" {
		if len(v) > maxTaskEventSeqParamBytes {
			return 0, fmt.Errorf("%w: limit too long", domain.ErrInvalidInput)
		}
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
