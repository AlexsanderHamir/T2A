package apijson

import (
	"context"
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
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "apijson.WriteJSONError")
	ApplySecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	ctx := requestOrBackgroundContext(r)
	body := buildJSONErrorBody(ctx, r, msg)
	payload, err := marshalJSONErrorBody(ctx, op, r, body)
	if err != nil {
		writeJSONEncodeFailure(w)
		return
	}
	debugLogJSONErrorOut(ctx, r, op, code, payload, callPath)
	w.WriteHeader(code)
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n"))
}
