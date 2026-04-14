package calltrace

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type stackKey struct{}

// Push returns ctx with name appended to the call stack used for call_path / helper.io logs.
// Use at handler entry (operation string) and at the start of nested helpers.
func Push(ctx context.Context, name string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	name = strings.TrimSpace(name)
	if name == "" {
		return ctx
	}
	var parent []string
	if s, ok := ctx.Value(stackKey{}).([]string); ok && s != nil {
		parent = s
	}
	next := make([]string, len(parent)+1)
	copy(next, parent)
	next[len(parent)] = name
	return context.WithValue(ctx, stackKey{}, next)
}

// Path returns "parent > child > ..." for the current ctx, or empty when unset.
func Path(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	s, _ := ctx.Value(stackKey{}).([]string)
	if len(s) == 0 {
		return ""
	}
	return strings.Join(s, " > ")
}

// WithRequestRoot attaches the HTTP handler operation as the first stack frame (after middleware context).
func WithRequestRoot(r *http.Request, op string) *http.Request {
	if r == nil {
		return nil
	}
	_ = slog.Default().Enabled(r.Context(), slog.LevelDebug)
	return r.WithContext(Push(r.Context(), op))
}
