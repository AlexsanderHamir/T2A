package apijson

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

type jsonErrorBody struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteJSONError writes {"error":"msg","request_id":"..."} when the request context carries an id.
// Security + JSON content-type headers match the main API. When callPath is non-nil and Debug is
// enabled, emits the same http.io debug shape as handler paths (including call_path when known).
func WriteJSONError(w http.ResponseWriter, r *http.Request, op string, code int, msg string, callPath func(context.Context) string) {
	ApplySecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	body := jsonErrorBody{Error: msg}
	if r != nil {
		if rid := logctx.RequestIDFromContext(ctx); rid != "" {
			body.RequestID = rid
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		slog.Error("response encode failed", "cmd", logctx.TraceCmd, "operation", op, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "{\"error\":\"internal server error\"}\n")
		return
	}
	payload := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if r != nil && slog.Default().Enabled(ctx, slog.LevelDebug) {
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
	w.WriteHeader(code)
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n"))
}
