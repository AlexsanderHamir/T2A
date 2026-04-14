package taskapi

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const cmdLog = "taskapi"

// NewHTTPHandler returns the REST + SSE task API with the standard middleware stack
// (see handler.MiddlewareStack) wrapping handler.NewHandler.
func NewHTTPHandler(s *store.Store, hub *handler.SSEHub, rep *repo.Root) http.Handler {
	slog.Debug("trace", "cmd", cmdLog, "operation", "internal.taskapi.NewHTTPHandler")
	return handler.MiddlewareStack(handler.NewHandler(s, hub, rep))
}
