package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/apijson"
)

func TestWriteJSONWithETag_emits_etag_and_revalidatable_cache_control(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/x", nil)
	writeJSONWithETag(rr, req, "test.etag", http.StatusOK, map[string]any{"id": "x", "n": 1})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header missing on 200")
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag should be quoted, got %q", etag)
	}
	if got := rr.Header().Get("Cache-Control"); got != apijson.RevalidatableCacheControl {
		t.Errorf("Cache-Control = %q, want %q", got, apijson.RevalidatableCacheControl)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json...", got)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["id"] != "x" {
		t.Errorf("body[id] = %v, want x", body["id"])
	}
}

func TestWriteJSONWithETag_returns_304_when_if_none_match_matches(t *testing.T) {
	t.Parallel()
	first := httptest.NewRecorder()
	body := map[string]any{"id": "y", "v": 42}
	writeJSONWithETag(first, httptest.NewRequest(http.MethodGet, "/tasks/y", nil), "test.etag", http.StatusOK, body)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/y", nil)
	req.Header.Set("If-None-Match", etag)
	writeJSONWithETag(second, req, "test.etag", http.StatusOK, body)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", second.Code)
	}
	if got := second.Header().Get("ETag"); got != etag {
		t.Errorf("ETag on 304 = %q, want %q", got, etag)
	}
	if got := second.Header().Get("Cache-Control"); got != apijson.RevalidatableCacheControl {
		t.Errorf("Cache-Control on 304 = %q, want %q", got, apijson.RevalidatableCacheControl)
	}
	if second.Body.Len() != 0 {
		t.Errorf("304 response body must be empty, got %q", second.Body.String())
	}
}

func TestWriteJSONWithETag_returns_full_body_when_if_none_match_differs(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/z", nil)
	req.Header.Set("If-None-Match", `"deadbeef"`)
	writeJSONWithETag(rr, req, "test.etag", http.StatusOK, map[string]any{"id": "z"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Error("expected full body, got empty")
	}
}

func TestWriteJSONWithETag_emits_security_headers(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/h", nil)
	writeJSONWithETag(rr, req, "test.etag", http.StatusOK, map[string]any{"ok": true})

	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
}
