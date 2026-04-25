package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// Task routes: see README.md (handler_task_*.go). /repo: repo_handlers.go. SSE: sse.go.
// Settings routes: handler_settings.go (GET/PATCH /settings, POST /settings/probe-cursor,
// POST /settings/list-cursor-models, POST /settings/cancel-current-run).

// AgentWorkerControl is the narrow surface the /settings handlers use
// to drive the in-process agent worker. The cmd/taskapi supervisor
// implements it; tests can stub it out (or pass nil to disable the
// supervisor-aware endpoints — they then return 503).
//
// Reload is invoked after PATCH /settings persists so the worker
// picks up the new config without a process restart. CancelCurrentRun
// is the explicit "stop the runaway run" knob exposed at
// POST /settings/cancel-current-run; it returns true when there was
// an in-flight run to cancel. ProbeRunner is invoked from POST
// /settings/probe-cursor so the SPA can validate a binary path
// against the configured runner before saving.
type AgentWorkerControl interface {
	CancelCurrentRun() bool
	Reload(ctx context.Context) error
	ProbeRunner(ctx context.Context, runnerID, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error)
}

// Handler carries dependencies for the mounted REST routes, SSE stream, repo
// helpers, and optional agent worker control. Use NewHandler; the zero value
// is not usable.
type Handler struct {
	store          *store.Store
	hub            *SSEHub
	repoProv       RepoProvider
	agent          AgentWorkerControl
	systemHealthFn systemHealthSnapshotter
}

// NewHandler returns the task REST API and GET /events (SSE) when hub is non-nil.
//
// rep is the legacy static workspace root: pass nil to disable /repo
// routes (they return 409 repo_root_not_configured) or pre-open one
// for tests that want a fixed tmpdir. The production wiring should
// instead pass nil here and call WithRepoProvider with a settings-
// backed provider so the repo follows AppSettings.RepoRoot live.
//
// agent is optional: when nil, settings-control endpoints (PATCH /settings,
// POST /settings/probe-cursor, POST /settings/cancel-current-run) respond 503.
// GET /settings still works without it (read-only).
func NewHandler(s *store.Store, hub *SSEHub, rep *repo.Root, opts ...HandlerOption) http.Handler {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewHandler")
	h := &Handler{store: s, hub: hub, repoProv: NewStaticRepoProvider(rep)}
	for _, opt := range opts {
		opt(h)
	}
	m := http.NewServeMux()
	m.Handle("GET /health", http.HandlerFunc(health))
	m.Handle("GET /health/live", http.HandlerFunc(healthLive))
	m.Handle("GET /health/ready", http.HandlerFunc(h.healthReady))
	m.Handle("GET /system/health", http.HandlerFunc(h.systemHealth))
	m.Handle("GET /events", http.HandlerFunc(h.streamEvents))
	m.Handle("POST /tasks", http.HandlerFunc(h.create))
	m.Handle("POST /tasks/evaluate", http.HandlerFunc(h.evaluateDraft))
	m.Handle("GET /task-drafts", http.HandlerFunc(h.listTaskDrafts))
	m.Handle("POST /task-drafts", http.HandlerFunc(h.saveTaskDraft))
	m.Handle("GET /task-drafts/{id}", http.HandlerFunc(h.getTaskDraft))
	m.Handle("DELETE /task-drafts/{id}", http.HandlerFunc(h.deleteTaskDraft))
	m.Handle("GET /tasks", http.HandlerFunc(h.list))
	m.Handle("GET /tasks/stats", http.HandlerFunc(h.stats))
	m.Handle("GET /tasks/cycle-failures", http.HandlerFunc(h.cycleFailures))
	m.Handle("GET /tasks/{id}/checklist", http.HandlerFunc(h.getChecklist))
	m.Handle("POST /tasks/{id}/checklist/items", http.HandlerFunc(h.postChecklistItem))
	m.Handle("PATCH /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.patchChecklistItem))
	m.Handle("DELETE /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.deleteChecklistItem))
	m.Handle("GET /tasks/{id}/events/{seq}", http.HandlerFunc(h.taskEvent))
	m.Handle("PATCH /tasks/{id}/events/{seq}", http.HandlerFunc(h.patchTaskEventUserResponse))
	m.Handle("GET /tasks/{id}/events", http.HandlerFunc(h.taskEvents))
	m.Handle("POST /tasks/{id}/cycles", http.HandlerFunc(h.postTaskCycle))
	m.Handle("GET /tasks/{id}/cycles", http.HandlerFunc(h.getTaskCycles))
	m.Handle("GET /tasks/{id}/cycles/{cycleId}/stream", http.HandlerFunc(h.getTaskCycleStream))
	m.Handle("GET /tasks/{id}/cycles/{cycleId}", http.HandlerFunc(h.getTaskCycle))
	m.Handle("PATCH /tasks/{id}/cycles/{cycleId}", http.HandlerFunc(h.patchTaskCycle))
	m.Handle("POST /tasks/{id}/cycles/{cycleId}/phases", http.HandlerFunc(h.postTaskCyclePhase))
	m.Handle("PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}", http.HandlerFunc(h.patchTaskCyclePhase))
	m.Handle("GET /tasks/{id}", http.HandlerFunc(h.get))
	m.Handle("PATCH /tasks/{id}", http.HandlerFunc(h.patch))
	m.Handle("DELETE /tasks/{id}", http.HandlerFunc(h.delete))
	m.Handle("GET /repo/search", http.HandlerFunc(h.repoSearch))
	m.Handle("GET /repo/file", http.HandlerFunc(h.repoFile))
	m.Handle("GET /repo/validate-range", http.HandlerFunc(h.repoValidateRange))
	m.Handle("GET /settings", http.HandlerFunc(h.getSettings))
	m.Handle("PATCH /settings", http.HandlerFunc(h.patchSettings))
	m.Handle("POST /settings/probe-cursor", http.HandlerFunc(h.probeCursor))
	m.Handle("POST /settings/list-cursor-models", http.HandlerFunc(h.listCursorModels))
	m.Handle("POST /settings/cancel-current-run", http.HandlerFunc(h.cancelCurrentRun))
	// /v1/rum is the SPA-side Real User Monitoring beacon. Documented
	// in docs/SLOs.md; the browser ships batches via
	// `navigator.sendBeacon` so the server returns 204 with no body.
	// Rate-limited via the global per-IP middleware (WithRateLimit),
	// not separately, so a misbehaving SPA cannot amplify a load
	// incident into a metrics-storage bill.
	m.Handle("POST /v1/rum", http.HandlerFunc(h.postRUM))
	return m
}

// HandlerOption configures the Handler at construction time. Optional
// because most callers (tests, embedding) only need the core surface.
type HandlerOption func(*Handler)

// WithAgentWorkerControl wires the supervisor that owns the in-process
// agent worker so PATCH /settings can hot-reload, POST
// /settings/probe-cursor can probe the runner, and POST
// /settings/cancel-current-run can cancel an in-flight run. Pass nil
// (or omit the option) to disable those endpoints — they then return
// 503 service_unavailable and GET /settings still works.
func WithAgentWorkerControl(c AgentWorkerControl) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithAgentWorkerControl")
	return func(h *Handler) {
		h.agent = c
	}
}

// WithRepoProvider replaces the default static repo wiring with a
// dynamic provider. cmd/taskapi passes a NewSettingsRepoProvider so
// /repo/* and prompt-mention validation always look at the current
// AppSettings.RepoRoot; tests rarely need this option (the rep
// argument to NewHandler covers the static case).
func WithRepoProvider(p RepoProvider) HandlerOption {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.WithRepoProvider")
	return func(h *Handler) {
		if p != nil {
			h.repoProv = p
		}
	}
}
