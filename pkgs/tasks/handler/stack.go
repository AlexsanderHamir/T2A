package handler

import (
	"log/slog"
	"net/http"
)

// MiddlewareStack wraps inner with the standard taskapi HTTP middleware chain.
//
// Order (outer → inner): recovery, HTTP metrics, access log, rate limit, optional bearer auth,
// request timeout, max body, idempotency.
func MiddlewareStack(inner http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.MiddlewareStack")
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
			),
		),
	)
}
