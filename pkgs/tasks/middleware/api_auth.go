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
//
// The auth-scheme name is matched case-insensitively per RFC 7235 § 2.1
// ("the case-insensitive token defined as the auth-scheme name") /
// RFC 6750 § 2.1 — curl, Postman, several reverse proxies, and a number
// of HTTP client libraries normalize the scheme to lowercase
// ("bearer ...") and the historical strings.HasPrefix(rawAuth,
// "Bearer ") check rejected every non-titlecase variant as 401 even
// when the credential matched (see
// TestHasValidBearerToken_caseInsensitiveScheme). The credential
// portion is still constant-time-compared for length and content.
func HasValidBearerToken(rawAuth, configuredToken string) bool {
	slog.Debug("trace", "cmd", logctx.TraceCmd, "operation", "middleware.HasValidBearerToken")
	rawAuth = strings.TrimSpace(rawAuth)
	if rawAuth == "" {
		return false
	}
	const prefix = "Bearer "
	if len(rawAuth) < len(prefix) || !strings.EqualFold(rawAuth[:len(prefix)], prefix) {
		return false
	}
	presented := strings.TrimSpace(rawAuth[len(prefix):])
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
