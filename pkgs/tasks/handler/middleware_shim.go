package handler

import (
	"net/http"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithRecovery(h http.Handler) http.Handler { return middleware.WithRecovery(h) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithHTTPMetrics(h http.Handler) http.Handler { return middleware.WithHTTPMetrics(h) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithAccessLog(h http.Handler) http.Handler { return middleware.WithAccessLog(h, calltrace.Path) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithRateLimit(h http.Handler) http.Handler { return middleware.WithRateLimit(h) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithAPIAuth(h http.Handler) http.Handler { return middleware.WithAPIAuth(h) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithRequestTimeout(h http.Handler) http.Handler { return middleware.WithRequestTimeout(h) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithMaxRequestBody(h http.Handler) http.Handler { return middleware.WithMaxRequestBody(h) }

//funclogmeasure:skip category=re-export-wrapper reason="Thin middleware re-export; slog lives on pkgs/tasks/middleware implementation."
func WithIdempotency(h http.Handler) http.Handler { return middleware.WithIdempotency(h) }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RateLimitPerMinuteConfigured() int { return middleware.RateLimitPerMinuteConfigured() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func APIAuthEnabled() bool { return middleware.APIAuthEnabled() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func MaxRequestBodyBytesConfigured() int { return middleware.MaxRequestBodyBytesConfigured() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func RequestTimeout() time.Duration { return middleware.RequestTimeout() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func IdempotencyTTL() time.Duration { return middleware.IdempotencyTTL() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func IdempotencyCacheLimits() (int, int) { return middleware.IdempotencyCacheLimits() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func clearIdempotencyStateForTest() { middleware.ClearIdempotencyStateForTest() }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// HasValidBearerToken re-exports middleware bearer parsing for handler package tests.
func HasValidBearerToken(rawAuth, configuredToken string) bool {
	return middleware.HasValidBearerToken(rawAuth, configuredToken)
}
