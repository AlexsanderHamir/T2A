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
)

func TestRateLimitPerMinuteConfigured(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("T2A_RATE_LIMIT_PER_MIN", "")
		if g := RateLimitPerMinuteConfigured(); g != defaultRateLimitPerMin {
			t.Fatalf("got %d want %d", g, defaultRateLimitPerMin)
		}
	})
	t.Run("zero", func(t *testing.T) {
		t.Setenv("T2A_RATE_LIMIT_PER_MIN", "0")
		if g := RateLimitPerMinuteConfigured(); g != 0 {
			t.Fatalf("got %d want 0", g)
		}
	})
	t.Run("custom", func(t *testing.T) {
		t.Setenv("T2A_RATE_LIMIT_PER_MIN", "42")
		if g := RateLimitPerMinuteConfigured(); g != 42 {
			t.Fatalf("got %d want 42", g)
		}
	})
	t.Run("invalid_falls_back", func(t *testing.T) {
		t.Setenv("T2A_RATE_LIMIT_PER_MIN", "nope")
		if g := RateLimitPerMinuteConfigured(); g != defaultRateLimitPerMin {
			t.Fatalf("got %d want default", g)
		}
	})
}

func TestClientIPForRateLimit(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.1:54321"
	if got := clientIPForRateLimit(r); got != "192.0.2.1" {
		t.Fatalf("got %q", got)
	}
	r.RemoteAddr = "[::1]:1234"
	if got := clientIPForRateLimit(r); got != "::1" {
		t.Fatalf("got %q", got)
	}
}

func TestWithAccessLog_rateLimitWarn_carriesRequestID(t *testing.T) {
	t.Setenv("T2A_RATE_LIMIT_PER_MIN", "1")
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithAccessLog(WithRateLimit(inner))
	addr := "198.51.100.22:5555"

	req1 := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req1.RemoteAddr = addr
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req2.RemoteAddr = addr
	req2.Header.Set("X-Request-ID", "rid-rate-limit-beta")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: %d", rec2.Code)
	}

	var rateLine map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["msg"] == "rate limit exceeded" {
			rateLine = m
			break
		}
	}
	if rateLine == nil {
		t.Fatalf("no rate limit log in: %q", buf.String())
	}
	if got := rateLine["request_id"]; got != "rid-rate-limit-beta" {
		t.Fatalf("request_id: got %v want rid-rate-limit-beta", got)
	}
	if rateLine["operation"] != "http.rate_limit" {
		t.Fatalf("operation: %v", rateLine["operation"])
	}
}

func TestWithRateLimit_429(t *testing.T) {
	t.Setenv("T2A_RATE_LIMIT_PER_MIN", "3")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithRateLimit(inner)
	addr := "198.51.100.7:9999"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d: status %d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = addr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After %q", rec.Header().Get("Retry-After"))
	}
	assertBaselineSecurityHeaders(t, rec.Header())
}

func TestWithRateLimit_disabled(t *testing.T) {
	t.Setenv("T2A_RATE_LIMIT_PER_MIN", "0")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithRateLimit(inner)
	addr := "198.51.100.8:9999"
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d: %d", i, rec.Code)
		}
	}
}

func TestWithRateLimit_exemptHealthUnderTightLimit(t *testing.T) {
	t.Setenv("T2A_RATE_LIMIT_PER_MIN", "1")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := WithRateLimit(inner)
	addr := "198.51.100.9:9999"
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d: %d", i, rec.Code)
		}
	}
}
