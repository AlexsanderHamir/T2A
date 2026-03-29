package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskCreateJSON struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	InitialPrompt string          `json:"initial_prompt"`
	Status        domain.Status   `json:"status"`
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

type taskEventLine struct {
	Seq  int64            `json:"seq"`
	At   time.Time        `json:"at"`
	Type domain.EventType `json:"type"`
	By   domain.Actor     `json:"by"`
	Data json.RawMessage  `json:"data"`
}

type taskEventsResponse struct {
	TaskID string          `json:"task_id"`
	Events []taskEventLine `json:"events"`
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

func (h *Handler) taskEvents(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.events"
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := h.store.Get(r.Context(), id); err != nil {
		writeStoreError(w, op, err)
		return
	}
	evs, err := h.store.ListTaskEvents(r.Context(), id)
	if err != nil {
		writeStoreError(w, op, err)
		return
	}
	out := make([]taskEventLine, 0, len(evs))
	for _, e := range evs {
		data := json.RawMessage(e.Data)
		if len(data) == 0 {
			data = json.RawMessage(`{}`)
		}
		out = append(out, taskEventLine{
			Seq:  e.Seq,
			At:   e.At,
			Type: e.Type,
			By:   e.By,
			Data: data,
		})
	}
	writeJSON(w, op, http.StatusOK, taskEventsResponse{TaskID: id, Events: out})
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
