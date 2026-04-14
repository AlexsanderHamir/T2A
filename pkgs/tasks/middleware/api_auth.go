package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/apijson"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
)

// AuthorizationHeader is the HTTP header name checked for bearer tokens.
const AuthorizationHeader = "Authorization"

func apiTokenConfigured() string {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.apiTokenConfigured")
	return strings.TrimSpace(os.Getenv("T2A_API_TOKEN"))
}

// APIAuthEnabled reports whether API bearer-token auth is enabled.
func APIAuthEnabled() bool {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.APIAuthEnabled")
	return apiTokenConfigured() != ""
}

func omitAPIAuth(r *http.Request) bool {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.omitAPIAuth")
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/health", "/health/live", "/health/ready", "/metrics":
		return true
	default:
		return false
	}
}

// HasValidBearerToken reports whether rawAuth is a well-formed Bearer token matching configuredToken (constant-time compare).
func HasValidBearerToken(rawAuth, configuredToken string) bool {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.HasValidBearerToken")
	rawAuth = strings.TrimSpace(rawAuth)
	if rawAuth == "" {
		return false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(rawAuth, prefix) {
		return false
	}
	presented := strings.TrimSpace(strings.TrimPrefix(rawAuth, prefix))
	if presented == "" || configuredToken == "" {
		return false
	}
	if len(presented) != len(configuredToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(configuredToken)) == 1
}

// WithAPIAuth enforces Authorization: Bearer <token> when T2A_API_TOKEN is set.
// When the token is unset, the wrapper is a no-op.
// GET /health, /health/live, /health/ready, and /metrics are exempt.
func WithAPIAuth(h http.Handler) http.Handler {
	token := apiTokenConfigured()
	if token == "" {
		return h
	}
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.WithAPIAuth")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if omitAPIAuth(r) {
			h.ServeHTTP(w, r)
			return
		}
		if !HasValidBearerToken(r.Header.Get(AuthorizationHeader), token) {
			slog.Log(r.Context(), slog.LevelWarn, "api auth denied",
				"cmd", logctx.TraceCmd, "operation", "http.api_auth",
				"method", r.Method, "path", r.URL.Path)
			apijson.WriteJSONError(w, r, "http.api_auth", http.StatusUnauthorized, "unauthorized", nil)
			return
		}
		h.ServeHTTP(w, r)
	})
}
