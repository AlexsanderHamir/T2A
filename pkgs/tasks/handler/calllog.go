package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type callStackKey struct{}

// PushCall returns ctx with name appended to the call stack used for call_path / helper.io logs.
// Use at handler entry (operation string) and at the start of nested helpers.
func PushCall(ctx context.Context, name string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	name = strings.TrimSpace(name)
	if name == "" {
		return ctx
	}
	var parent []string
	if s, ok := ctx.Value(callStackKey{}).([]string); ok && s != nil {
		parent = s
	}
	next := make([]string, len(parent)+1)
	copy(next, parent)
	next[len(parent)] = name
	return context.WithValue(ctx, callStackKey{}, next)
}

// CallPath returns "parent > child > ..." for the current ctx, or empty when unset.
func CallPath(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	s, _ := ctx.Value(callStackKey{}).([]string)
	if len(s) == 0 {
		return ""
	}
	return strings.Join(s, " > ")
}

// withCallRoot attaches the HTTP handler operation as the first stack frame (after middleware context).
func withCallRoot(r *http.Request, op string) *http.Request {
	if r == nil {
		return nil
	}
	_ = slog.Default().Enabled(r.Context(), slog.LevelDebug)
	return r.WithContext(PushCall(r.Context(), op))
}

func helperDebugIn(ctx context.Context, fn string, kv ...any) {
	if ctx == nil || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		"cmd", httpLogCmd,
		"obs_category", "helper_io",
		"call_path", CallPath(ctx),
		"function", fn,
		"phase", "helper_in",
	}
	args = append(args, kv...)
	slog.Log(ctx, slog.LevelDebug, "helper.io", args...)
}

func helperDebugOut(ctx context.Context, fn string, kv ...any) {
	if ctx == nil || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		"cmd", httpLogCmd,
		"obs_category", "helper_io",
		"call_path", CallPath(ctx),
		"function", fn,
		"phase", "helper_out",
	}
	args = append(args, kv...)
	slog.Log(ctx, slog.LevelDebug, "helper.io", args...)
}
