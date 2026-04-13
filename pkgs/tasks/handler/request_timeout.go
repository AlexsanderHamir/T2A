package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultRequestTimeout = 30 * time.Second
	requestTimeoutEnv     = "T2A_HTTP_REQUEST_TIMEOUT"
)

func requestTimeoutConfigured() time.Duration {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.requestTimeoutConfigured")
	raw := strings.TrimSpace(os.Getenv(requestTimeoutEnv))
	if raw == "" {
		return defaultRequestTimeout
	}
	if raw == "0" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return defaultRequestTimeout
	}
	return d
}

// RequestTimeout returns the effective request execution timeout.
// Default is 30s, invalid values fall back to 30s, and 0 disables.
func RequestTimeout() time.Duration {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.RequestTimeout")
	return requestTimeoutConfigured()
}

func omitRequestTimeout(r *http.Request) bool {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.omitRequestTimeout")
	return r.Method == http.MethodGet && r.URL.Path == "/events"
}

// WithRequestTimeout applies a context deadline to request execution.
// GET /events is exempt so SSE streams stay open.
func WithRequestTimeout(h http.Handler) http.Handler {
	timeout := requestTimeoutConfigured()
	if timeout <= 0 {
		return h
	}
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithRequestTimeout", "timeout_sec", int(timeout/time.Second))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if omitRequestTimeout(r) {
			h.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
