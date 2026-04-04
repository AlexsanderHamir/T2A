package handler

import (
	"context"
	"log/slog"
)

// WrapSlogHandlerWithRequestContext returns h wrapped so each record includes request_id when
// the log is emitted with a context from WithAccessLog (or any context carrying a request id).
// GORM SQL traces use the same context as store calls, so they correlate with HTTP requests.
func WrapSlogHandlerWithRequestContext(h slog.Handler) slog.Handler {
	if h == nil {
		return nil
	}
	return &requestContextSlogHandler{inner: h}
}

type requestContextSlogHandler struct {
	inner slog.Handler
}

func (w *requestContextSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return w.inner.Enabled(ctx, level)
}

func (w *requestContextSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		r.Add(slog.String("request_id", id))
	}
	return w.inner.Handle(ctx, r)
}

func (w *requestContextSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &requestContextSlogHandler{inner: w.inner.WithAttrs(attrs)}
}

func (w *requestContextSlogHandler) WithGroup(name string) slog.Handler {
	return &requestContextSlogHandler{inner: w.inner.WithGroup(name)}
}
