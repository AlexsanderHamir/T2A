package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

// WithRecovery wraps h so panics are logged and answered with a JSON 500 response.
func WithRecovery(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.WithRecovery")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = resolveAndAttachRequestID(w, r)
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				path := ""
				if r.URL != nil {
					path = r.URL.Path
				}
				route := path
				if r.Pattern != "" {
					route = r.Pattern
				}
				rid := logctx.RequestIDFromContext(r.Context())
				slog.Log(r.Context(), slog.LevelError, "panic in handler",
					"cmd", logctx.TraceCmd, "operation", "http.recover",
					"request_id", rid,
					"method", r.Method, "path", path, "route", route,
					"duration_ms", time.Since(start).Milliseconds(),
					"panic", rec, "stack", debug.Stack())
				apijson.WriteJSONError(w, r, "http.recover", http.StatusInternalServerError, "internal server error", nil)
			}
		}()
		h.ServeHTTP(w, r)
	})
}
