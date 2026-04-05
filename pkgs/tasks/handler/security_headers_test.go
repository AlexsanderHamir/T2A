package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func assertBaselineSecurityHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if got := h.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q want DENY", got)
	}
	if got := h.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q want no-referrer", got)
	}
	if got := h.Get("Content-Security-Policy"); got == "" || got != "default-src 'none'; frame-ancestors 'none'" {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q want nosniff", got)
	}
	wantPP := "camera=(), microphone=(), geolocation=(), payment=()"
	if got := h.Get("Permissions-Policy"); got != wantPP {
		t.Errorf("Permissions-Policy = %q want %q", got, wantPP)
	}
}

func TestHTTP_health_includes_security_headers(t *testing.T) {
	srv := newTaskTestServer(t)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertBaselineSecurityHeaders(t, res.Header)
}

func TestStreamEvents_sets_security_headers(t *testing.T) {
	db := testdb.OpenSQLite(t)
	h := &Handler{store: store.NewStore(db), hub: NewSSEHub(), repo: nil}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.streamEvents(rec, req)
	assertBaselineSecurityHeaders(t, rec.Header())
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
}
