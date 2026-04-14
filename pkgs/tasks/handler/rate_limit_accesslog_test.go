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
