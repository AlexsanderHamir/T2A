package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrapSlogHandlerWithRequestContext_addsRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h)
	ctx := ContextWithRequestID(context.Background(), "corr-test-1")
	lg.Log(ctx, slog.LevelInfo, "probe", "k", "v")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if got := line["request_id"]; got != "corr-test-1" {
		t.Fatalf("request_id %v", got)
	}
}

func TestWrapSlogHandlerWithRequestContext_noIDWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	h := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h)
	lg.Log(context.Background(), slog.LevelInfo, "probe")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if _, ok := line["request_id"]; ok {
		t.Fatal("unexpected request_id")
	}
}

func TestWithAccessLog_echoesXRequestIDAndLogsAccess(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) == "" {
			t.Fatal("expected request id in context")
		}
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("x"))
	})

	srv := httptest.NewServer(WithAccessLog(inner))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Request-ID", "client-rid-99")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("X-Request-ID"); got != "client-rid-99" {
		t.Fatalf("echo X-Request-ID: %q", got)
	}
	if _, err := io.Copy(io.Discard, res.Body); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 1 {
		t.Fatalf("want access log line, got %d bytes: %q", buf.Len(), buf.String())
	}

	var access map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &access); err != nil {
		t.Fatal(err)
	}
	if access["msg"] != "http request complete" {
		t.Fatalf("last line msg: %v", access["msg"])
	}
	if access["request_id"] != "client-rid-99" {
		t.Fatalf("request_id: %v", access["request_id"])
	}
	if access["operation"] != "http.access" {
		t.Fatalf("operation: %v", access["operation"])
	}
	if access["method"] != "GET" {
		t.Fatalf("method: %v", access["method"])
	}
	if int(access["status"].(float64)) != http.StatusTeapot {
		t.Fatalf("status: %v", access["status"])
	}
}

func TestWithAccessLog_skipsHealth(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(WithAccessLog(inner))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if strings.Contains(buf.String(), "http request complete") {
		t.Fatalf("health should not log access: %q", buf.String())
	}
}

func TestRequestIDFromContext_empty(t *testing.T) {
	if RequestIDFromContext(nil) != "" {
		t.Fatal("nil ctx")
	}
	if RequestIDFromContext(context.Background()) != "" {
		t.Fatal("background")
	}
}
