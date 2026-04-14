// Package logctx provides request-scoped context (request id, per-request log sequence)
// and slog.Handler wrappers for correlating JSON logs in taskapi.
//
// It depends only on the standard library so cmd/taskapi and pkgs/tasks/handler can
// both import it without import cycles.
package logctx
