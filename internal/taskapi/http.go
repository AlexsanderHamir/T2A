package taskapi

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const cmdLog = "taskapi"

// NewHTTPHandler returns the REST + SSE task API with the standard middleware stack
// (see pkgs/tasks/middleware.Stack) wrapping handler.NewHandler.
//
// rep is the legacy static workspace; pass nil in production wiring
// to delegate to the settings-backed RepoProvider built inside
// (which makes /repo/* + prompt mention validation follow
// AppSettings.RepoRoot live, as required by docs/SETTINGS.md).
// Tests that need a fixed tmpdir can still pass a non-nil rep.
//
// Pass a nil agent control to opt out of the supervisor-aware
// /settings sub-routes (PATCH /settings, POST /settings/probe-cursor,
// POST /settings/cancel-current-run); GET /settings still works.
func NewHTTPHandler(s *store.Store, hub *handler.SSEHub, rep *repo.Root, agent handler.AgentWorkerControl) http.Handler {
	slog.Debug("trace", "cmd", cmdLog, "operation", "internal.taskapi.NewHTTPHandler")
	opts := []handler.HandlerOption{}
	if agent != nil {
		opts = append(opts, handler.WithAgentWorkerControl(agent))
	}
	if rep == nil {
		opts = append(opts, handler.WithRepoProvider(handler.NewSettingsRepoProvider(s)))
	}
	return middleware.Stack(handler.NewHandler(s, hub, rep, opts...), calltrace.Path)
}
