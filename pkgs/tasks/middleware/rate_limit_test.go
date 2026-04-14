package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
	hd := rec.Header()
	if got := hd.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q want no-store", got)
	}
	if got := hd.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q want DENY", got)
	}
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
