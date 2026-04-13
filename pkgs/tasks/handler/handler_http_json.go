package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

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

// applyAPISecurityHeaders sets baseline hardening headers without logging.
// High-frequency paths (for example Prometheus scrapes) use this alone; JSON uses it from setJSONHeaders
// so each response does not emit both setJSONHeaders and setAPISecurityHeaders trace lines.
func applyAPISecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
}

// setAPISecurityHeaders sets baseline hardening headers for browser-facing HTTP responses (SSE, plain-text errors, idempotency replay).
func setAPISecurityHeaders(w http.ResponseWriter) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.setAPISecurityHeaders")
	applyAPISecurityHeaders(w)
}

// WrapPrometheusHandler applies the same baseline response hardening as API routes
// (see applyAPISecurityHeaders) before delegating to the Prometheus registry handler.
// Scrapers ignore these headers; they help when /metrics is opened in a browser.
// Per-scrape debug trace is omitted so metrics polling does not flood logs at level debug.
func WrapPrometheusHandler(next http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WrapPrometheusHandler")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyAPISecurityHeaders(w)
		next.ServeHTTP(w, r)
	})
}

func setJSONHeaders(w http.ResponseWriter) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.setJSONHeaders")
	applyAPISecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

type jsonErrorBody struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

// writeJSON writes v as JSON. When r is non-nil and Debug is enabled, logs response_body (truncated) and response_json_bytes.
func writeJSON(w http.ResponseWriter, r *http.Request, op string, code int, v any) {
	setJSONHeaders(w)
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		slog.Error("response encode failed", "cmd", httpLogCmd, "operation", op, "err", err)
		writeJSONError(w, r, op, http.StatusInternalServerError, "internal server error")
		return
	}
	payload := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if r != nil && slog.Default().Enabled(ctx, slog.LevelDebug) {
		preview := truncateUTF8ByBytes(string(payload), maxHTTPLogJSONPreviewBytes)
		slog.Log(ctx, slog.LevelDebug, "http.io",
			"cmd", httpLogCmd, "obs_category", "http_io", "operation", op, "call_path", CallPath(ctx), "phase", "out",
			"http_status", code, "response_json_bytes", len(payload), "response_body", preview)
	}
	w.WriteHeader(code)
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n"))
}

func writeJSONError(w http.ResponseWriter, r *http.Request, op string, code int, msg string) {
	setJSONHeaders(w)
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	body := jsonErrorBody{Error: msg}
	if r != nil {
		if rid := RequestIDFromContext(ctx); rid != "" {
			body.RequestID = rid
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		slog.Error("response encode failed", "cmd", httpLogCmd, "operation", op, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "{\"error\":\"internal server error\"}\n")
		return
	}
	payload := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if r != nil && slog.Default().Enabled(ctx, slog.LevelDebug) {
		preview := truncateUTF8ByBytes(string(payload), maxHTTPLogJSONPreviewBytes)
		slog.Log(ctx, slog.LevelDebug, "http.io",
			"cmd", httpLogCmd, "obs_category", "http_io", "operation", op, "call_path", CallPath(ctx), "phase", "out",
			"http_status", code, "response_json_bytes", len(payload), "response_body", preview)
	}
	w.WriteHeader(code)
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n"))
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
	case errors.Is(err, context.DeadlineExceeded):
		code = http.StatusGatewayTimeout
		msg = "request timed out"
		return code, msg
	case errors.Is(err, context.Canceled):
		code = http.StatusRequestTimeout
		msg = "request canceled"
		return code, msg
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
