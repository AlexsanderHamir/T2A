package handler

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// WithRecovery wraps h so panics are logged and answered with a JSON 500 response.
func WithRecovery(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithRecovery")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Log(r.Context(), slog.LevelError, "panic in handler",
					"cmd", httpLogCmd, "operation", "http.recover", "panic", rec, "stack", debug.Stack())
				writeJSONError(w, r, "http.recover", http.StatusInternalServerError, "internal server error")
			}
		}()
		h.ServeHTTP(w, r)
	})
}
