// Package httpsecurityexpect holds assertions for baseline browser-facing HTTP
// response headers (see docs/API-HTTP.md). It must not import pkgs/tasks/handler
// so handler tests and internal/handlertest can both depend on it without cycles.
package httpsecurityexpect

import (
	"net/http"
	"testing"
)

// AssertBaselineHeaders checks the same baseline hardening as production JSON,
// SSE, and metrics responses.
func AssertBaselineHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q want no-store", got)
	}
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
