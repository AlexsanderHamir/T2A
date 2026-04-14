package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/google/uuid"
)

const maxHTTPLogQueryBytes = 1024

// WithAccessLog wraps h to assign a request id, attach it to r.Context, echo X-Request-ID on
// responses, and emit one structured line per request when it finishes (method, path, route,
// status, duration, bytes written). GET /health, /health/live, and /health/ready skip the access
// line to avoid probe noise but still assign and echo X-Request-ID (and request context) so probes
// and any logs during readiness stay correlatable.
//
// callPath may be nil; when non-nil it supplies call_path on the access line (e.g. pkgs/tasks/calltrace.Path).
func WithAccessLog(h http.Handler, callPath func(context.Context) string) http.Handler {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.WithAccessLog")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = resolveAndAttachRequestID(w, r)

		if omitAccessLog(r) {
			h.ServeHTTP(w, r)
			return
		}

		ctx := logctx.ContextWithLogSeq(r.Context())
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
			q = apijson.TruncateUTF8ByBytes(q, maxHTTPLogQueryBytes)
		}
		cp := ""
		if callPath != nil {
			cp = callPath(ctx)
		}
		slog.Log(ctx, slog.LevelInfo, "http request complete",
			"cmd", logctx.TraceCmd,
			"obs_category", "http_access",
			"operation", "http.access",
			"call_path", cp,
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
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.resolveAndAttachRequestID")
	if id := logctx.RequestIDFromContext(r.Context()); id != "" {
		w.Header().Set("X-Request-ID", id)
		return r
	}
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if id == "" {
		id = uuid.NewString()
	} else if len(id) > logctx.MaxIncomingRequestIDLen {
		id = id[:logctx.MaxIncomingRequestIDLen]
	}
	w.Header().Set("X-Request-ID", id)
	ctx := logctx.ContextWithRequestID(r.Context(), id)
	return r.WithContext(ctx)
}

func omitAccessLog(r *http.Request) bool {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.omitAccessLog")
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
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.accessLogWriter.WriteHeader")
	if !aw.wroteHeader {
		aw.status = code
		aw.wroteHeader = true
	}
	aw.ResponseWriter.WriteHeader(code)
}

func (aw *accessLogWriter) Write(b []byte) (int, error) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.accessLogWriter.Write")
	if !aw.wroteHeader {
		aw.status = http.StatusOK
		aw.wroteHeader = true
	}
	n, err := aw.ResponseWriter.Write(b)
	aw.bytes += int64(n)
	return n, err
}

func (aw *accessLogWriter) Flush() {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.accessLogWriter.Flush")
	if f, ok := aw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var _ http.Flusher = (*accessLogWriter)(nil)
