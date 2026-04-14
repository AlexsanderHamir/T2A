// Package middleware provides the standard HTTP middleware chain for taskapi (recovery,
// metrics, access logging, rate limit, auth, timeouts, body cap, idempotency).
//
// It depends on apijson and logctx only — not on pkgs/tasks/handler — so internal/taskapi
// can call Stack(handler.NewHandler(...), calltrace.Path) without import cycles. callPath
// for access logs is injected from the caller (typically calltrace.Path).
package middleware
