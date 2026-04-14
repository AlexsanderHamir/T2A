package handler

import (
	"net/http"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

func WithRecovery(h http.Handler) http.Handler { return middleware.WithRecovery(h) }

func WithHTTPMetrics(h http.Handler) http.Handler { return middleware.WithHTTPMetrics(h) }

func WithAccessLog(h http.Handler) http.Handler { return middleware.WithAccessLog(h, calltrace.Path) }

func WithRateLimit(h http.Handler) http.Handler { return middleware.WithRateLimit(h) }

func WithAPIAuth(h http.Handler) http.Handler { return middleware.WithAPIAuth(h) }

func WithRequestTimeout(h http.Handler) http.Handler { return middleware.WithRequestTimeout(h) }

func WithMaxRequestBody(h http.Handler) http.Handler { return middleware.WithMaxRequestBody(h) }

func WithIdempotency(h http.Handler) http.Handler { return middleware.WithIdempotency(h) }

func RateLimitPerMinuteConfigured() int { return middleware.RateLimitPerMinuteConfigured() }

func APIAuthEnabled() bool { return middleware.APIAuthEnabled() }

func MaxRequestBodyBytesConfigured() int { return middleware.MaxRequestBodyBytesConfigured() }

func RequestTimeout() time.Duration { return middleware.RequestTimeout() }

func IdempotencyTTL() time.Duration { return middleware.IdempotencyTTL() }

func IdempotencyCacheLimits() (int, int) { return middleware.IdempotencyCacheLimits() }

func clearIdempotencyStateForTest() { middleware.ClearIdempotencyStateForTest() }

// HasValidBearerToken re-exports middleware bearer parsing for handler package tests.
func HasValidBearerToken(rawAuth, configuredToken string) bool {
	return middleware.HasValidBearerToken(rawAuth, configuredToken)
}
