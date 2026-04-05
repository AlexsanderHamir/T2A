package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// Task routes: handler_tasks.go. /repo: repo_handlers.go. SSE: sse.go.

const httpLogCmd = "taskapi"

type Handler struct {
	store *store.Store
	hub   *SSEHub
	repo  *repo.Root
}

// NewHandler returns the task REST API and GET /events (SSE) when hub is non-nil.
// rep is optional: when nil, /repo routes return 503 and initial_prompt is not validated for file mentions.
func NewHandler(s *store.Store, hub *SSEHub, rep *repo.Root) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.NewHandler")
	h := &Handler{store: s, hub: hub, repo: rep}
	m := http.NewServeMux()
	m.Handle("GET /health", http.HandlerFunc(health))
	m.Handle("GET /health/live", http.HandlerFunc(healthLive))
	m.Handle("GET /health/ready", http.HandlerFunc(h.healthReady))
	m.Handle("GET /events", http.HandlerFunc(h.streamEvents))
	m.Handle("POST /tasks", http.HandlerFunc(h.create))
	m.Handle("GET /tasks", http.HandlerFunc(h.list))
	m.Handle("GET /tasks/{id}/checklist", http.HandlerFunc(h.getChecklist))
	m.Handle("POST /tasks/{id}/checklist/items", http.HandlerFunc(h.postChecklistItem))
	m.Handle("PATCH /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.patchChecklistItem))
	m.Handle("DELETE /tasks/{id}/checklist/items/{itemId}", http.HandlerFunc(h.deleteChecklistItem))
	m.Handle("GET /tasks/{id}/events/{seq}", http.HandlerFunc(h.taskEvent))
	m.Handle("PATCH /tasks/{id}/events/{seq}", http.HandlerFunc(h.patchTaskEventUserResponse))
	m.Handle("GET /tasks/{id}/events", http.HandlerFunc(h.taskEvents))
	m.Handle("GET /tasks/{id}", http.HandlerFunc(h.get))
	m.Handle("PATCH /tasks/{id}", http.HandlerFunc(h.patch))
	m.Handle("DELETE /tasks/{id}", http.HandlerFunc(h.delete))
	m.Handle("GET /repo/search", http.HandlerFunc(h.repoSearch))
	m.Handle("GET /repo/validate-range", http.HandlerFunc(h.repoValidateRange))
	return m
}

func health(w http.ResponseWriter, r *http.Request) {
	writeLiveness(w, r, "health")
}

func healthLive(w http.ResponseWriter, r *http.Request) {
	writeLiveness(w, r, "health.live")
}

func writeLiveness(w http.ResponseWriter, r *http.Request, op string) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler."+op)
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op)
	writeJSON(w, r, op, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": ServerVersion(),
	})
}

const healthReadyDBTimeout = 2 * time.Second

func (h *Handler) healthReady(w http.ResponseWriter, r *http.Request) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.health.ready")
	const op = "health.ready"
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op)
	ctx, cancel := context.WithTimeout(r.Context(), healthReadyDBTimeout)
	defer cancel()

	checks := map[string]string{}

	if err := h.store.Ready(ctx); err != nil {
		slog.Warn("readiness check failed", "cmd", httpLogCmd, "operation", op, "check", "database", "err", err,
			"deadline_exceeded", errors.Is(err, context.DeadlineExceeded),
			"timeout_sec", int(healthReadyDBTimeout/time.Second))
		checks["database"] = "fail"
		writeJSON(w, r, op, http.StatusServiceUnavailable, map[string]any{
			"status":  "degraded",
			"checks":  checks,
			"version": ServerVersion(),
		})
		return
	}
	checks["database"] = "ok"

	if h.repo != nil {
		if err := h.repo.Ready(); err != nil {
			slog.Warn("readiness check failed", "cmd", httpLogCmd, "operation", op, "check", "workspace_repo", "err", err)
			checks["workspace_repo"] = "fail"
			writeJSON(w, r, op, http.StatusServiceUnavailable, map[string]any{
				"status":  "degraded",
				"checks":  checks,
				"version": ServerVersion(),
			})
			return
		}
		checks["workspace_repo"] = "ok"
	}

	writeJSON(w, r, op, http.StatusOK, map[string]any{
		"status":  "ok",
		"checks":  checks,
		"version": ServerVersion(),
	})
}

func actorFromRequest(r *http.Request) (a domain.Actor) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.actorFromRequest")
	if r == nil {
		return domain.ActorUser
	}
	ctx := PushCall(r.Context(), "actorFromRequest")
	raw := strings.TrimSpace(r.Header.Get("X-Actor"))
	helperDebugIn(ctx, "actorFromRequest", "x_actor_raw", raw)
	defer func() {
		helperDebugOut(ctx, "actorFromRequest", "actor", string(a))
	}()
	switch strings.ToLower(raw) {
	case "agent":
		return domain.ActorAgent
	default:
		return domain.ActorUser
	}
}

func decodeJSON(ctx context.Context, r io.Reader, dst any) (err error) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.decodeJSON")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = PushCall(ctx, "decodeJSON")
	helperDebugIn(ctx, "decodeJSON", "dst_type", fmt.Sprintf("%T", dst))
	defer func() { helperDebugOut(ctx, "decodeJSON", "err", err) }()
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err = dec.Decode(dst); err != nil {
		err = fmt.Errorf("json decode: %w", err)
		return err
	}
	if err = dec.Decode(&struct{}{}); err != nil {
		if err == io.EOF {
			err = nil
			return nil
		}
		err = fmt.Errorf("json trailing data: %w", err)
		return err
	}
	err = fmt.Errorf("%w: json trailing data", domain.ErrInvalidInput)
	return err
}

// setAPISecurityHeaders sets baseline hardening headers for browser-facing HTTP responses (JSON, SSE, and plain-text errors).
func setAPISecurityHeaders(w http.ResponseWriter) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.setAPISecurityHeaders")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
}

func setJSONHeaders(w http.ResponseWriter) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.setJSONHeaders")
	setAPISecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

type jsonErrorBody struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

// writeJSON writes v as JSON. When r is non-nil and Debug is enabled, logs response_body (truncated) and response_json_bytes.
func writeJSON(w http.ResponseWriter, r *http.Request, op string, code int, v any) {
	setJSONHeaders(w)
	w.WriteHeader(code)
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	if r != nil && slog.Default().Enabled(ctx, slog.LevelDebug) {
		b, err := json.Marshal(v)
		if err != nil {
			slog.Error("response marshal failed", "cmd", httpLogCmd, "operation", op, "err", err)
			return
		}
		preview := truncateUTF8ByBytes(string(b), maxHTTPLogJSONPreviewBytes)
		slog.Log(ctx, slog.LevelDebug, "http.io",
			"cmd", httpLogCmd, "obs_category", "http_io", "operation", op, "call_path", CallPath(ctx), "phase", "out",
			"http_status", code, "response_json_bytes", len(b), "response_body", preview)
		if _, werr := w.Write(b); werr != nil {
			return
		}
		_, _ = w.Write([]byte("\n"))
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		slog.Error("response encode failed", "cmd", httpLogCmd, "operation", op, "err", err)
	}
}

func writeJSONError(w http.ResponseWriter, r *http.Request, op string, code int, msg string) {
	setJSONHeaders(w)
	w.WriteHeader(code)
	body := jsonErrorBody{Error: msg}
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
		if rid := RequestIDFromContext(ctx); rid != "" {
			body.RequestID = rid
		}
	}
	if r != nil && slog.Default().Enabled(ctx, slog.LevelDebug) {
		b, err := json.Marshal(body)
		if err != nil {
			slog.Error("response marshal failed", "cmd", httpLogCmd, "operation", op, "err", err)
			return
		}
		preview := truncateUTF8ByBytes(string(b), maxHTTPLogJSONPreviewBytes)
		slog.Log(ctx, slog.LevelDebug, "http.io",
			"cmd", httpLogCmd, "obs_category", "http_io", "operation", op, "call_path", CallPath(ctx), "phase", "out",
			"http_status", code, "response_json_bytes", len(b), "response_body", preview)
		if _, werr := w.Write(b); werr != nil {
			return
		}
		_, _ = w.Write([]byte("\n"))
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		slog.Error("response encode failed", "cmd", httpLogCmd, "operation", op, "err", err)
	}
}

func userFacingJSONError(err error) string {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.userFacingJSONError")
	s := err.Error()
	if strings.HasPrefix(s, "json decode: ") {
		return strings.TrimPrefix(s, "json decode: ")
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		return "request body must contain a single JSON value"
	}
	if strings.HasPrefix(s, "json trailing data:") {
		return "request body must contain a single JSON value"
	}
	return s
}

func storeErrorClientMessage(err error) string {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.storeErrorClientMessage")
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "not found"
	case errors.Is(err, domain.ErrConflict):
		return "task id already exists"
	case errors.Is(err, domain.ErrInvalidInput):
		if d := invalidInputDetail(err); d != "" {
			return d
		}
		return "bad request"
	default:
		return "internal server error"
	}
}

func invalidInputDetail(err error) string {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.invalidInputDetail")
	s := err.Error()
	const mark = "tasks: invalid input: "
	if i := strings.Index(s, mark); i >= 0 {
		return strings.TrimSpace(s[i+len(mark):])
	}
	return ""
}

func writeError(w http.ResponseWriter, r *http.Request, op string, err error, code int) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.writeError", "http_op", op)
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		code = http.StatusRequestEntityTooLarge
	}
	ctxErr := PushCall(requestCtx(r), "writeError")
	helperDebugIn(ctxErr, "writeError", "http_op", op, "http_status", code, "err", err)
	logRequestFailure(requestCtx(r), op, err, code)
	msg := http.StatusText(code)
	switch code {
	case http.StatusRequestEntityTooLarge:
		msg = "request body too large"
	case http.StatusBadRequest:
		msg = userFacingJSONError(err)
		if msg == "" {
			msg = "bad request"
		}
	}
	helperDebugOut(ctxErr, "writeError", "client_facing_msg", msg)
	writeJSONError(w, r, op, code, msg)
}

// storeErrHTTPResponse maps store/domain errors to an HTTP status and JSON error body message.
func storeErrHTTPResponse(ctx context.Context, err error) (code int, msg string) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.storeErrHTTPResponse")
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = PushCall(ctx, "storeErrHTTPResponse")
	helperDebugIn(ctx, "storeErrHTTPResponse", "err", err)
	defer func() {
		helperDebugOut(ctx, "storeErrHTTPResponse", "http_status", code, "client_msg", msg)
	}()
	code = http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidInput):
		code = http.StatusBadRequest
	case errors.Is(err, domain.ErrConflict):
		code = http.StatusConflict
	}
	msg = storeErrorClientMessage(err)
	if code == http.StatusInternalServerError {
		msg = "internal server error"
	}
	return code, msg
}

func writeStoreError(w http.ResponseWriter, r *http.Request, op string, err error) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.writeStoreError", "http_op", op)
	ctxErr := PushCall(requestCtx(r), "writeStoreError")
	helperDebugIn(ctxErr, "writeStoreError", "http_op", op, "err", err)
	code, msg := storeErrHTTPResponse(ctxErr, err)
	helperDebugOut(ctxErr, "writeStoreError", "http_status", code, "client_facing_msg", msg)
	logRequestFailure(requestCtx(r), op, err, code)
	writeJSONError(w, r, op, code, msg)
}

func requestCtx(r *http.Request) context.Context {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.requestCtx")
	if r == nil {
		return context.Background()
	}
	return r.Context()
}

func logRequestFailure(ctx context.Context, op string, err error, httpStatus int) {
	attrs := []any{"cmd", httpLogCmd, "operation", op, "call_path", CallPath(ctx), "http_status", httpStatus, "err", err}
	if httpStatus >= 500 {
		slog.Log(ctx, slog.LevelError, "request failed", attrs...)
		return
	}
	slog.Log(ctx, slog.LevelWarn, "request failed", attrs...)
}
