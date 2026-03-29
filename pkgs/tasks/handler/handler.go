package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const httpLogCmd = "taskapi"

type Handler struct {
	store *store.Store
	hub   *SSEHub
	repo  *repo.Root
}

// NewHandler returns the task REST API and GET /events (SSE) when hub is non-nil.
// rep is optional: when nil, /repo routes return 503 and initial_prompt is not validated for file mentions.
func NewHandler(s *store.Store, hub *SSEHub, rep *repo.Root) http.Handler {
	h := &Handler{store: s, hub: hub, repo: rep}
	m := http.NewServeMux()
	m.Handle("GET /events", http.HandlerFunc(h.streamEvents))
	m.Handle("POST /tasks", http.HandlerFunc(h.create))
	m.Handle("GET /tasks", http.HandlerFunc(h.list))
	m.Handle("GET /tasks/{id}", http.HandlerFunc(h.get))
	m.Handle("PATCH /tasks/{id}", http.HandlerFunc(h.patch))
	m.Handle("DELETE /tasks/{id}", http.HandlerFunc(h.delete))
	m.Handle("GET /repo/search", http.HandlerFunc(h.repoSearch))
	m.Handle("GET /repo/validate-range", http.HandlerFunc(h.repoValidateRange))
	return m
}

type taskCreateJSON struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	InitialPrompt string         `json:"initial_prompt"`
	Status        domain.Status  `json:"status"`
	Priority      domain.Priority `json:"priority"`
}

type taskPatchJSON struct {
	Title         *string          `json:"title"`
	InitialPrompt *string          `json:"initial_prompt"`
	Status        *domain.Status   `json:"status"`
	Priority      *domain.Priority `json:"priority"`
}

type listResponse struct {
	Tasks  []domain.Task `json:"tasks"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.create"
	var body taskCreateJSON
	if err := decodeJSON(r.Body, &body); err != nil {
		writeError(w, op, err, http.StatusBadRequest)
		return
	}
	if h.repo != nil {
		if err := h.repo.ValidatePromptMentions(body.InitialPrompt); err != nil {
			writeJSONError(w, op, http.StatusBadRequest, err.Error())
			return
		}
	}
	by := actorFromRequest(r)
	t, err := h.store.Create(r.Context(), store.CreateTaskInput{
		ID:            body.ID,
		Title:         body.Title,
		InitialPrompt: body.InitialPrompt,
		Status:        body.Status,
		Priority:      body.Priority,
	}, by)
	if err != nil {
		writeStoreError(w, op, err)
		return
	}
	h.notifyChange(TaskCreated, t.ID)
	writeJSON(w, op, http.StatusCreated, t)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.get"
	id := strings.TrimSpace(r.PathValue("id"))
	t, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, op, err)
		return
	}
	writeJSON(w, op, http.StatusOK, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.list"
	limit, offset, err := parseListParams(r.URL.Query())
	if err != nil {
		writeStoreError(w, op, err)
		return
	}
	tasks, err := h.store.List(r.Context(), limit, offset)
	if err != nil {
		writeStoreError(w, op, err)
		return
	}
	writeJSON(w, op, http.StatusOK, listResponse{Tasks: tasks, Limit: limit, Offset: offset})
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.patch"
	id := strings.TrimSpace(r.PathValue("id"))
	var body taskPatchJSON
	if err := decodeJSON(r.Body, &body); err != nil {
		writeError(w, op, err, http.StatusBadRequest)
		return
	}
	in := store.UpdateTaskInput{
		Title:         body.Title,
		InitialPrompt: body.InitialPrompt,
		Status:        body.Status,
		Priority:      body.Priority,
	}
	if h.repo != nil && body.InitialPrompt != nil {
		if err := h.repo.ValidatePromptMentions(*body.InitialPrompt); err != nil {
			writeJSONError(w, op, http.StatusBadRequest, err.Error())
			return
		}
	}
	by := actorFromRequest(r)
	t, err := h.store.Update(r.Context(), id, in, by)
	if err != nil {
		writeStoreError(w, op, err)
		return
	}
	h.notifyChange(TaskUpdated, t.ID)
	writeJSON(w, op, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.delete"
	id := strings.TrimSpace(r.PathValue("id"))
	if err := h.store.Delete(r.Context(), id); err != nil {
		writeStoreError(w, op, err)
		return
	}
	h.notifyChange(TaskDeleted, id)
	w.WriteHeader(http.StatusNoContent)
}

func parseListParams(q url.Values) (limit, offset int, err error) {
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

func actorFromRequest(r *http.Request) domain.Actor {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("X-Actor"))) {
	case "agent":
		return domain.ActorAgent
	default:
		return domain.ActorUser
	}
}

func decodeJSON(r io.Reader, dst any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("json decode: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("json trailing data: %w", err)
	}
	return fmt.Errorf("%w: json trailing data", domain.ErrInvalidInput)
}

func writeJSON(w http.ResponseWriter, op string, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		slog.Error("response encode failed", "cmd", httpLogCmd, "operation", op, "err", err)
	}
}

func writeError(w http.ResponseWriter, op string, err error, code int) {
	logRequestFailure(op, err, code)
	http.Error(w, http.StatusText(code), code)
}

func writeStoreError(w http.ResponseWriter, op string, err error) {
	msg := "internal server error"
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		msg = "not found"
		code = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidInput):
		msg = "bad request"
		code = http.StatusBadRequest
	}
	logRequestFailure(op, err, code)
	http.Error(w, msg, code)
}

func logRequestFailure(op string, err error, httpStatus int) {
	attrs := []any{"cmd", httpLogCmd, "operation", op, "http_status", httpStatus, "err", err}
	if httpStatus >= 500 {
		slog.Error("request failed", attrs...)
		return
	}
	slog.Warn("request failed", attrs...)
}
