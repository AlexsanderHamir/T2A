package calltrace

import (
	"context"
	"net/http"
	"strings"
)

type stackKey struct{}

// Push returns ctx with name appended to the call stack used for call_path /
// helper.io logs. Use at handler entry (operation string) and at the start
// of nested helpers.
//
// Nil-ctx contract: callers MUST NOT pre-guard `if ctx == nil { ctx =
// context.Background() }` before calling Push. Push handles nil internally
// (line 17 below) and the result is always a non-nil context that is safe
// to pass to HelperIOIn / HelperIOOut (which also nil-guard internally via
// helperDebugIn / helperDebugOut). Several handler-side parsers used to
// carry a redundant pre-guard before Session 28 — that pattern is now
// obsolete and should not be re-introduced.
//
// Skip-listed in cmd/funclogmeasure/analyze.go: pure context-mutation
// helper called once per handler/helper entry, where the caller is already
// responsible for emitting the surrounding trace line.
func Push(ctx context.Context, name string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
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

// Path returns "parent > child > ..." for the current ctx, or empty when
// unset. Skip-listed: pure context-read helper consumed by callers that
// embed the result into their own trace line.
func Path(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(stackKey{}).([]string)
	if len(s) == 0 {
		return ""
	}
	return strings.Join(s, " > ")
}

// WithRequestRoot attaches the HTTP handler operation as the first stack
// frame (after middleware context). Skip-listed: thin one-line wrapper
// over Push; the per-request access log emitted by the access middleware
// already names the operation.
func WithRequestRoot(r *http.Request, op string) *http.Request {
	if r == nil {
		return nil
	}
	return r.WithContext(Push(r.Context(), op))
}
