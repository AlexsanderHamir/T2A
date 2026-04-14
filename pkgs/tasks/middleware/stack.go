package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

// Stack wraps inner with the standard taskapi HTTP middleware chain.
//
// Order (outer → inner): recovery, HTTP metrics, access log, rate limit, optional bearer auth,
// request timeout, max body, idempotency.
//
// callPath is used for access log call_path (typically pkgs/tasks/calltrace.Path from internal/taskapi); it may be nil.
func Stack(inner http.Handler, callPath func(context.Context) string) http.Handler {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.Stack")
	return WithRecovery(
		WithHTTPMetrics(
			WithAccessLog(
				WithRateLimit(
					WithAPIAuth(
						WithRequestTimeout(
							WithMaxRequestBody(
								WithIdempotency(inner),
							),
						),
					),
				),
				callPath,
			),
		),
	)
}
