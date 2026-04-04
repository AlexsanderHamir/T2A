package handler

import (
	"context"
)

type ctxKey int

const ctxKeyRequestID ctxKey = 1

const maxIncomingRequestIDLen = 128

// ContextWithRequestID returns ctx with the HTTP request id attached for slog and correlation.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
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
	s, _ := ctx.Value(ctxKeyRequestID).(string)
	return s
}
