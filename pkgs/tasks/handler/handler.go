package handler

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// Task routes: handler_tasks.go. /repo: repo_handlers.go. SSE: sse.go.

const httpLogCmd = "taskapi"

type Handler struct {
	store *store.Store
	hub   *SSEHub
	repo  *repo.Root
	// userTaskAgents is optional: when set, user-originated POST /tasks enqueues a task snapshot after persistence.
	userTaskAgents agents.UserTaskCreatedNotifier
}

// HandlerOption configures NewHandler.
type HandlerOption func(*Handler)

// WithUserTaskAgentNotifier registers n for user-created tasks (see pkgs/agents). When nil, the option is a no-op.
func WithUserTaskAgentNotifier(n agents.UserTaskCreatedNotifier) HandlerOption {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithUserTaskAgentNotifier")
	return func(h *Handler) {
		if h == nil || n == nil {
			return
		}
		h.userTaskAgents = n
	}
}

// NewHandler returns the task REST API and GET /events (SSE) when hub is non-nil.
// rep is optional: when nil, /repo routes return 503 and initial_prompt is not validated for file mentions.
func NewHandler(s *store.Store, hub *SSEHub, rep *repo.Root, opts ...HandlerOption) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.NewHandler")
	h := &Handler{store: s, hub: hub, repo: rep}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	m := http.NewServeMux()
	m.Handle("GET /health", http.HandlerFunc(health))
	m.Handle("GET /health/live", http.HandlerFunc(healthLive))
	m.Handle("GET /health/ready", http.HandlerFunc(h.healthReady))
	m.Handle("GET /events", http.HandlerFunc(h.streamEvents))
	m.Handle("POST /tasks", http.HandlerFunc(h.create))
	m.Handle("POST /tasks/evaluate", http.HandlerFunc(h.evaluateDraft))
	m.Handle("GET /task-drafts", http.HandlerFunc(h.listTaskDrafts))
	m.Handle("POST /task-drafts", http.HandlerFunc(h.saveTaskDraft))
	m.Handle("GET /task-drafts/{id}", http.HandlerFunc(h.getTaskDraft))
	m.Handle("DELETE /task-drafts/{id}", http.HandlerFunc(h.deleteTaskDraft))
	m.Handle("GET /tasks", http.HandlerFunc(h.list))
	m.Handle("GET /tasks/stats", http.HandlerFunc(h.stats))
	m.Handle("GET /tasks/{id}/checklist", http.HandlerFunc(h.getChecklist))
	m.Handle("POST /tasks/{id}/checklist/items", http.HandlerFunc(h.postChecklistItem))
	m.Handle("PATCH /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.patchChecklistItem))
	m.Handle("DELETE /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.deleteChecklistItem))
	m.Handle("GET /tasks/{id}/events/{seq}", http.HandlerFunc(h.taskEvent))
	m.Handle("PATCH /tasks/{id}/events/{seq}", http.HandlerFunc(h.patchTaskEventUserResponse))
	m.Handle("GET /tasks/{id}/events", http.HandlerFunc(h.taskEvents))
	m.Handle("GET /tasks/{id}", http.HandlerFunc(h.get))
	m.Handle("PATCH /tasks/{id}", http.HandlerFunc(h.patch))
	m.Handle("DELETE /tasks/{id}", http.HandlerFunc(h.delete))
	m.Handle("GET /repo/search", http.HandlerFunc(h.repoSearch))
	m.Handle("GET /repo/file", http.HandlerFunc(h.repoFile))
	m.Handle("GET /repo/validate-range", http.HandlerFunc(h.repoValidateRange))
	return m
}
