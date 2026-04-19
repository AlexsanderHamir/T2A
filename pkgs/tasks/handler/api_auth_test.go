package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

func TestAPIAuthEnabled(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "")
	if APIAuthEnabled() {
		t.Fatal("expected auth disabled")
	}
	t.Setenv("T2A_API_TOKEN", "secret")
	if !APIAuthEnabled() {
		t.Fatal("expected auth enabled")
	}
}

func TestHasValidBearerToken(t *testing.T) {
	if HasValidBearerToken("", "secret") {
		t.Fatal("empty header should fail")
	}
	if HasValidBearerToken("secret", "secret") {
		t.Fatal("missing bearer prefix should fail")
	}
	if HasValidBearerToken("Bearer ", "secret") {
		t.Fatal("empty bearer should fail")
	}
	if HasValidBearerToken("Bearer nope", "secret") {
		t.Fatal("wrong token should fail")
	}
	if !HasValidBearerToken("Bearer secret", "secret") {
		t.Fatal("valid token should pass")
	}
}

func TestWithAccessLog_apiAuthDenied_logIncludesRequestID(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "secret")

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithAccessLog(WithAPIAuth(inner))

	req := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	req.Header.Set("X-Request-ID", "rid-api-auth-deny")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var warnLine map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["msg"] == "api auth denied" {
			warnLine = m
			break
		}
	}
	if warnLine == nil {
		t.Fatalf("no warn log in %q", buf.String())
	}
	if warnLine["request_id"] != "rid-api-auth-deny" {
		t.Fatalf("request_id: %v", warnLine["request_id"])
	}
}

func TestWithAPIAuth_unauthorized_without_token(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithAPIAuth(inner)

	req := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithAPIAuth_authorized_with_bearer_token(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithAPIAuth(inner)

	req := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	req.Header.Set(middleware.AuthorizationHeader, "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

// TestHasValidBearerToken_caseInsensitiveScheme pins RFC 7235 § 2.1:
// the auth-scheme name is case-insensitive. curl, Postman, and several
// proxies normalize the scheme to lowercase ("bearer ..."), and other
// stacks emit ALL-CAPS. The historical strings.HasPrefix(rawAuth,
// "Bearer ") check rejected every non-titlecase variant as 401 even
// though the credential matched, breaking integrations that lowercased
// the scheme name.
func TestHasValidBearerToken_caseInsensitiveScheme(t *testing.T) {
	for _, scheme := range []string{"bearer", "BEARER", "Bearer", "BeArEr"} {
		if !HasValidBearerToken(scheme+" secret", "secret") {
			t.Errorf("scheme %q should be accepted (RFC 7235 §2.1: scheme names are case-insensitive); got reject", scheme)
		}
	}
	if HasValidBearerToken("Basic secret", "secret") {
		t.Error("Basic scheme must still be rejected even when the credential matches; got accept")
	}
}

func TestWithAPIAuth_authorized_with_lowercase_bearer_scheme(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithAPIAuth(inner)

	req := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	req.Header.Set(middleware.AuthorizationHeader, "bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lowercase bearer scheme: status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestWithAPIAuth_exempts_health_and_metrics(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithAPIAuth(inner)

	for _, path := range []string{"/health", "/health/live", "/health/ready", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path %s status %d", path, rec.Code)
		}
	}
}
