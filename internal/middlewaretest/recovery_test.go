package middlewaretest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

func TestWithRecovery_Returns500JSONOnPanic(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("intentional test panic")
	})
	srv := httptest.NewServer(middleware.WithRecovery(h))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "internal server error" {
		t.Fatalf("error body %q", body.Error)
	}
}

func TestWithRecovery_logsMethodAndPathOnPanic(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("intentional")
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/tasks/x1", nil)
	middleware.WithRecovery(h).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
	out := buf.String()
	if !strings.Contains(out, `"method":"PATCH"`) {
		t.Fatalf("missing method in log: %s", out)
	}
	if !strings.Contains(out, `"path":"/tasks/x1"`) {
		t.Fatalf("missing path in log: %s", out)
	}
	if !strings.Contains(out, `"operation":"http.recover"`) {
		t.Fatalf("missing operation: %s", out)
	}
}
