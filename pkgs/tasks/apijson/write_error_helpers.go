package apijson

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

func requestOrBackgroundContext(r *http.Request) context.Context {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "apijson.requestOrBackgroundContext")
	if r == nil {
		return context.Background()
	}
	return r.Context()
}

func buildJSONErrorBody(ctx context.Context, r *http.Request, msg string) jsonErrorBody {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "apijson.buildJSONErrorBody")
	body := jsonErrorBody{Error: msg}
	if r != nil {
		if rid := logctx.RequestIDFromContext(ctx); rid != "" {
			body.RequestID = rid
		}
	}
	return body
}

func marshalJSONErrorBody(ctx context.Context, op string, r *http.Request, body jsonErrorBody) ([]byte, error) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "apijson.marshalJSONErrorBody")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		logJSONEncodeError(ctx, op, r, err)
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func logJSONEncodeError(ctx context.Context, op string, r *http.Request, err error) {
	rid := ""
	route := ""
	method := ""
	path := ""
	if r != nil {
		rid = logctx.RequestIDFromContext(ctx)
		method = r.Method
		if r.URL != nil {
			path = r.URL.Path
		}
		if r.Pattern != "" {
			route = r.Pattern
		} else {
			route = path
		}
	}
	slog.Log(ctx, slog.LevelError, "response encode failed",
		"cmd", logctx.TraceCmd, "operation", op,
		"request_id", rid, "method", strings.TrimSpace(method), "path", path, "route", route,
		"err", err)
}

func writeJSONEncodeFailure(w http.ResponseWriter) {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "apijson.writeJSONEncodeFailure")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = io.WriteString(w, "{\"error\":\"internal server error\"}\n")
}

func debugLogJSONErrorOut(ctx context.Context, r *http.Request, op string, code int, payload []byte, callPath func(context.Context) string) {
	if r == nil || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	preview := TruncateUTF8ByBytes(string(payload), maxJSONLogPreviewBytes)
	args := []any{
		"cmd", logctx.TraceCmd,
		"obs_category", "http_io",
		"operation", op,
		"phase", "out",
		"http_status", code,
		"response_json_bytes", len(payload),
		"response_body", preview,
	}
	if callPath != nil {
		args = append(args, "call_path", callPath(ctx))
	}
	slog.Log(ctx, slog.LevelDebug, "http.io", args...)
}
