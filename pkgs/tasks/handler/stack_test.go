package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

// middleware.Stack with calltrace.Path is the production chain for taskapi; these tests pin critical
// behavior without requiring a real store or Postgres. Do not use t.Parallel with
// t.Setenv here.
func TestMiddlewareStack_innerPanic_returnsJSON500(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "")
	t.Setenv("T2A_RATE_LIMIT_PER_MIN", "0")

	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("intentional test panic")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__middleware_stack_test__/panic", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	middleware.Stack(inner, calltrace.Path).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type: got %q, want application/json prefix", ct)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if body.Error != "internal server error" {
		t.Fatalf("error message: got %q, want %q", body.Error, "internal server error")
	}
}

func TestMiddlewareStack_innerOK(t *testing.T) {
	t.Setenv("T2A_API_TOKEN", "")
	t.Setenv("T2A_RATE_LIMIT_PER_MIN", "0")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "short-circuit")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__middleware_stack_test__/ok", nil)
	req.RemoteAddr = "127.0.0.1:12346"

	middleware.Stack(inner, calltrace.Path).ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusTeapot)
	}
	b, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(b) != "short-circuit" {
		t.Fatalf("body: got %q, want %q", b, "short-circuit")
	}
}
