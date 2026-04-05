package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const maxRequestBodyEnv = "T2A_MAX_REQUEST_BODY_BYTES"

// MaxRequestBodyBytesConfigured returns the max request body size from T2A_MAX_REQUEST_BODY_BYTES.
// 0 or unset means no limit (default). Invalid or negative values are treated as 0 (unlimited).
func MaxRequestBodyBytesConfigured() int {
	s := strings.TrimSpace(os.Getenv(maxRequestBodyEnv))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// WithMaxRequestBody rejects bodies larger than the configured limit with 413 (JSON error body).
// When the limit is 0, the wrapper is a no-op. Uses Content-Length when present for an early reject,
// and http.MaxBytesReader so unknown or undersized Content-Length cannot bypass the cap.
func WithMaxRequestBody(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithMaxRequestBody")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		max := MaxRequestBodyBytesConfigured()
		if max <= 0 {
			h.ServeHTTP(w, r)
			return
		}
		ml := int64(max)
		if r.ContentLength > ml {
			slog.Warn("request body over limit", "cmd", httpLogCmd, "operation", "handler.max_body",
				"limit", max, "content_length", r.ContentLength)
			writeJSONError(w, r, "http.max_body", http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, ml)
		}
		h.ServeHTTP(w, r)
	})
}
