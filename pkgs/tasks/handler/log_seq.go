package handler

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type logSeqKey struct{}

// ContextWithLogSeq attaches a per-request monotonic counter. Every slog record emitted with this
// context gets a rising log_seq (via WrapSlogHandlerWithLogSequence), so JSON lines for one
// request can be sorted by log_seq to recover call order.
func ContextWithLogSeq(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	return context.WithValue(ctx, logSeqKey{}, new(atomic.Uint64))
}

func logSeqFromContext(ctx context.Context) *atomic.Uint64 {
	c := ctx
	if c == nil {
		c = context.Background()
	}
	_ = slog.Default().Enabled(c, slog.LevelDebug)
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(logSeqKey{}).(*atomic.Uint64)
	return v
}

// WrapSlogHandlerWithLogSequence adds log_seq (and log_seq_scope) to each record. When ctx carries
// a counter from ContextWithLogSeq, scope is "request". Otherwise processFallback is incremented
// and scope is "process" (startup / health / background) so non-request lines still have order.
func WrapSlogHandlerWithLogSequence(h slog.Handler, processFallback *atomic.Uint64) slog.Handler {
	_ = slog.Default().Enabled(context.Background(), slog.LevelDebug)
	if h == nil {
		return nil
	}
	return &logSeqSlogHandler{inner: h, processFallback: processFallback}
}

type logSeqSlogHandler struct {
	inner           slog.Handler
	processFallback *atomic.Uint64
}

func (w *logSeqSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return w.inner.Enabled(ctx, level)
}

func (w *logSeqSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if p := logSeqFromContext(ctx); p != nil {
		n := p.Add(1)
		r.Add(slog.Uint64("log_seq", n), slog.String("log_seq_scope", "request"))
	} else if w.processFallback != nil {
		n := w.processFallback.Add(1)
		r.Add(slog.Uint64("log_seq", n), slog.String("log_seq_scope", "process"))
	}
	return w.inner.Handle(ctx, r)
}

func (w *logSeqSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &logSeqSlogHandler{inner: w.inner.WithAttrs(attrs), processFallback: w.processFallback}
}

func (w *logSeqSlogHandler) WithGroup(name string) slog.Handler {
	return &logSeqSlogHandler{inner: w.inner.WithGroup(name), processFallback: w.processFallback}
}
