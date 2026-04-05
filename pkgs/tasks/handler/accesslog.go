package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WithAccessLog wraps h to assign a request id, attach it to r.Context, echo X-Request-ID on
// responses, and emit one structured line per request when it finishes (method, path, route,
// status, duration, bytes written). GET /health, /health/live, and /health/ready skip the access
// line to avoid probe noise but still assign and echo X-Request-ID (and request context) so probes
// and any logs during readiness stay correlatable.
func WithAccessLog(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithAccessLog")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = resolveAndAttachRequestID(w, r)

		if omitAccessLog(r) {
			h.ServeHTTP(w, r)
			return
		}

		ctx := ContextWithLogSeq(r.Context())
		r = r.WithContext(ctx)

		aw := &accessLogWriter{ResponseWriter: w}
		start := time.Now()

		h.ServeHTTP(aw, r)

		dur := time.Since(start)
		status := aw.status
		if status == 0 {
			status = http.StatusOK
		}
		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		q := r.URL.RawQuery
		if len(q) > maxHTTPLogQueryBytes {
			q = truncateUTF8ByBytes(q, maxHTTPLogQueryBytes)
		}
		slog.Log(ctx, slog.LevelInfo, "http request complete",
			"cmd", httpLogCmd,
			"obs_category", "http_access",
			"operation", "http.access",
			"call_path", CallPath(ctx),
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"query", q,
			"x_actor", strings.TrimSpace(r.Header.Get("X-Actor")),
			"status", status,
			"duration_ms", dur.Milliseconds(),
			"bytes_written", aw.bytes,
		)
	})
}

func resolveAndAttachRequestID(w http.ResponseWriter, r *http.Request) *http.Request {
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if id == "" {
		id = uuid.NewString()
	} else if len(id) > maxIncomingRequestIDLen {
		id = id[:maxIncomingRequestIDLen]
	}
	w.Header().Set("X-Request-ID", id)
	ctx := ContextWithRequestID(r.Context(), id)
	return r.WithContext(ctx)
}

func omitAccessLog(r *http.Request) bool {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.omitAccessLog")
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/health", "/health/live", "/health/ready":
		return true
	default:
		return false
	}
}

type accessLogWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	bytes       int64
}

func (aw *accessLogWriter) WriteHeader(code int) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.accessLogWriter.WriteHeader")
	if !aw.wroteHeader {
		aw.status = code
		aw.wroteHeader = true
	}
	aw.ResponseWriter.WriteHeader(code)
}

func (aw *accessLogWriter) Write(b []byte) (int, error) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.accessLogWriter.Write")
	if !aw.wroteHeader {
		aw.status = http.StatusOK
		aw.wroteHeader = true
	}
	n, err := aw.ResponseWriter.Write(b)
	aw.bytes += int64(n)
	return n, err
}

func (aw *accessLogWriter) Flush() {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.accessLogWriter.Flush")
	if f, ok := aw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var _ http.Flusher = (*accessLogWriter)(nil)
