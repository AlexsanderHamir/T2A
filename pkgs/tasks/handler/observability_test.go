package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/google/uuid"
)

func TestWithAccessLog_echoesXRequestIDAndLogsAccess(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logctx.RequestIDFromContext(r.Context()) == "" {
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
	if access["obs_category"] != "http_access" {
		t.Fatalf("obs_category: %v", access["obs_category"])
	}
	if access["log_seq_scope"] != "request" {
		t.Fatalf("log_seq_scope: %v", access["log_seq_scope"])
	}
	if int(access["log_seq"].(float64)) != 1 {
		t.Fatalf("log_seq: %v", access["log_seq"])
	}
}

func TestWithAccessLog_skipsHealth(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/health/live", "/health/ready":
			if logctx.RequestIDFromContext(r.Context()) == "" {
				t.Fatalf("missing request id for %s", r.URL.Path)
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(WithAccessLog(inner))
	t.Cleanup(srv.Close)

	for _, path := range []string{"/health", "/health/live", "/health/ready"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}

	if strings.Contains(buf.String(), "http request complete") {
		t.Fatalf("health probes should not log access: %q", buf.String())
	}

	reqProbe, err := http.NewRequest(http.MethodGet, srv.URL+"/health/ready", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqProbe.Header.Set("X-Request-ID", "probe-correlation-1")
	resProbe, err := http.DefaultClient.Do(reqProbe)
	if err != nil {
		t.Fatal(err)
	}
	defer resProbe.Body.Close()
	if got := strings.TrimSpace(resProbe.Header.Get("X-Request-ID")); got != "probe-correlation-1" {
		t.Fatalf("health echo X-Request-ID: %q", got)
	}
	_, _ = io.Copy(io.Discard, resProbe.Body)

	reqGen, err := http.NewRequest(http.MethodGet, srv.URL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	resGen, err := http.DefaultClient.Do(reqGen)
	if err != nil {
		t.Fatal(err)
	}
	defer resGen.Body.Close()
	if rid := strings.TrimSpace(resGen.Header.Get("X-Request-ID")); rid == "" {
		t.Fatal("health missing X-Request-ID")
	} else if _, err := uuid.Parse(rid); err != nil {
		t.Fatalf("health X-Request-ID not a UUID: %q", rid)
	}
	_, _ = io.Copy(io.Discard, resGen.Body)
}

func TestWithAccessLog_FlushDelegatesToUnderlyingFlusher(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, nil))))

	rec := httptest.NewRecorder()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("access log wrapper should expose http.Flusher")
		}
		w.WriteHeader(http.StatusOK)
		f.Flush()
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	WithAccessLog(inner).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWithAccessLog_truncatesLongXRequestID(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))))

	long := strings.Repeat("x", logctx.MaxIncomingRequestIDLen+40)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-ID", long)
	WithAccessLog(inner).ServeHTTP(rec, req)

	echo := rec.Header().Get("X-Request-ID")
	if len(echo) != logctx.MaxIncomingRequestIDLen {
		t.Fatalf("echo len %d want %d", len(echo), logctx.MaxIncomingRequestIDLen)
	}
	if echo != strings.Repeat("x", logctx.MaxIncomingRequestIDLen) {
		t.Fatal("truncation should preserve prefix")
	}
}

func TestLogSSEWriteError_logsWhenClientContextActive(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))))

	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	r = r.WithContext(logctx.ContextWithRequestID(r.Context(), "sse-rid"))
	logSSEWriteError(r, "tasks.sse", errors.New("simulated write failure"))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line["msg"] != "sse write failed" {
		t.Fatalf("msg %v", line["msg"])
	}
	if line["request_id"] != "sse-rid" {
		t.Fatalf("request_id %v", line["request_id"])
	}
}

func TestLogSSEWriteError_skipsWhenRequestContextCanceled(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	r = r.WithContext(ctx)
	logSSEWriteError(r, "tasks.sse", errors.New("would log if not canceled"))

	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected no log, got %q", buf.String())
	}
}
