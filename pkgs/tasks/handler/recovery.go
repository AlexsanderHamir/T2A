package handler

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// WithRecovery wraps h so panics are logged and answered with a JSON 500 response.
func WithRecovery(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Log(r.Context(), slog.LevelError, "panic in handler",
					"cmd", httpLogCmd, "operation", "http.recover", "panic", rec, "stack", debug.Stack())
				writeJSONError(w, "http.recover", http.StatusInternalServerError, "internal server error")
			}
		}()
		h.ServeHTTP(w, r)
	})
}
