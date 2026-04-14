package taskapi

import (
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const cmdLog = "taskapi"

// NewHTTPHandler returns the REST + SSE task API with the standard middleware chain:
// recovery, HTTP metrics, access log, rate limit, optional bearer auth, request timeout,
// max body, idempotency — wrapping handler.NewHandler.
func NewHTTPHandler(s *store.Store, hub *handler.SSEHub, rep *repo.Root) http.Handler {
	slog.Debug("trace", "cmd", cmdLog, "operation", "internal.taskapi.NewHTTPHandler")
	inner := handler.NewHandler(s, hub, rep)
	return handler.WithRecovery(handler.WithHTTPMetrics(handler.WithAccessLog(handler.WithRateLimit(handler.WithAPIAuth(handler.WithRequestTimeout(handler.WithMaxRequestBody(handler.WithIdempotency(inner))))))))
}
