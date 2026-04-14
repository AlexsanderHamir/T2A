package logctx

import (
	"context"
	"log/slog"
)

type ctxKey int

const ctxKeyRequestID ctxKey = 1

// MaxIncomingRequestIDLen caps the length of an incoming X-Request-ID header (trimmed, then truncated).
const MaxIncomingRequestIDLen = 128

// ContextWithRequestID returns ctx with the HTTP request id attached for slog and correlation.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	slog.Debug("trace", "cmd", TraceCmd, "operation", "logctx.ContextWithRequestID")
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestIDFromContext returns the request id from ctx, or empty when unset.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	s, _ := ctx.Value(ctxKeyRequestID).(string)
	return s
}
