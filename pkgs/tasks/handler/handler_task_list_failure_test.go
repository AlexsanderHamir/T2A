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

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

func captureHandlerLogs(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))
	return &buf
}

func lastLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("want at least one log line, got %q", buf.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &line); err != nil {
		t.Fatalf("decode log: %v raw=%q", err, lines[len(lines)-1])
	}
	return line
}

// Regression (2026-06-05): closed DB on GET /tasks returned 500 but store_list
// pagination context was missing from structured logs.
func TestHTTP_list_storeFailure_logsPaginationContext(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelError)

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(NewHandler(st, NewSSEHub(), nil))
	t.Cleanup(srv.Close)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(srv.URL + "/tasks?limit=25&offset=10")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 500 body=%s", res.StatusCode, body)
	}

	line := lastLogLine(t, buf)
	if line["msg"] != "request failed" {
		t.Fatalf("msg=%v want request failed", line["msg"])
	}
	if line["operation"] != "tasks.list" {
		t.Fatalf("operation=%v", line["operation"])
	}
	if line["failure_stage"] != "store_list" {
		t.Fatalf("failure_stage=%v want store_list", line["failure_stage"])
	}
	if int(line["limit"].(float64)) != 25 {
		t.Fatalf("limit=%v want 25", line["limit"])
	}
	if int(line["offset"].(float64)) != 10 {
		t.Fatalf("offset=%v want 10", line["offset"])
	}
	if line["pagination_mode"] != "offset" {
		t.Fatalf("pagination_mode=%v want offset", line["pagination_mode"])
	}
}

// Regression (2026-06-20): keyset GET /tasks store failures must log pagination_mode=keyset.
func TestHTTP_list_keysetStoreFailure_logsKeysetContext(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelError)

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(NewHandler(st, NewSSEHub(), nil))
	t.Cleanup(srv.Close)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	afterID := uuid.New().String()
	res, err := http.Get(srv.URL + "/tasks?limit=10&after_id=" + afterID)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d want 500 body=%s", res.StatusCode, body)
	}

	line := lastLogLine(t, buf)
	if line["failure_stage"] != "store_list" {
		t.Fatalf("failure_stage=%v want store_list", line["failure_stage"])
	}
	if line["after_id"] != afterID {
		t.Fatalf("after_id=%v want %s", line["after_id"], afterID)
	}
	if line["pagination_mode"] != "keyset" {
		t.Fatalf("pagination_mode=%v want keyset", line["pagination_mode"])
	}
}

// Regression (2026-06-20): canceled list requests map to 408 with store_list context.
func TestHTTP_list_canceledContext_logsStoreList(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelWarn)

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	h := NewHandler(st, NewSSEHub(), nil)
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/tasks?limit=5", nil).WithContext(ctx)

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status %d want 408", rec.Code)
	}

	line := lastLogLine(t, buf)
	if line["failure_stage"] != "store_list" {
		t.Fatalf("failure_stage=%v want store_list", line["failure_stage"])
	}
	if int(line["http_status"].(float64)) != http.StatusRequestTimeout {
		t.Fatalf("http_status=%v want 408", line["http_status"])
	}
}

func TestHTTP_list_parseFailure_logsQueryEchoes(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelWarn)

	srv := newTaskTestServer(t)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/tasks?limit=999")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", res.StatusCode)
	}

	line := lastLogLine(t, buf)
	if line["msg"] != "request failed" {
		t.Fatalf("msg=%v want request failed", line["msg"])
	}
	if line["failure_stage"] != "parse_list_params" {
		t.Fatalf("failure_stage=%v want parse_list_params", line["failure_stage"])
	}
	if line["limit_q"] != "999" {
		t.Fatalf("limit_q=%v want 999", line["limit_q"])
	}
}

type errAfterHeaderWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (w *errAfterHeaderWriter) WriteHeader(code int) {
	w.headerWritten = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *errAfterHeaderWriter) Write(b []byte) (int, error) {
	if w.headerWritten {
		return 0, errors.New("simulated response write failure")
	}
	return w.ResponseWriter.Write(b)
}

func TestWriteJSON_responseWriteFailure_logs(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelError)

	inner := httptest.NewRecorder()
	w := &errAfterHeaderWriter{ResponseWriter: inner}
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = req.WithContext(logctx.ContextWithRequestID(req.Context(), "list-write-rid"))

	writeJSON(w, req, "tasks.list", http.StatusOK, map[string]any{"tasks": []any{}})

	line := lastLogLine(t, buf)
	if line["msg"] != "response write failed" {
		t.Fatalf("msg=%v want response write failed", line["msg"])
	}
	if line["operation"] != "tasks.list" {
		t.Fatalf("operation=%v", line["operation"])
	}
	if line["failure_stage"] != "body" {
		t.Fatalf("failure_stage=%v want body", line["failure_stage"])
	}
	if line["request_id"] != "list-write-rid" {
		t.Fatalf("request_id=%v", line["request_id"])
	}
}

func TestWriteJSONWithETag_responseWriteFailure_logs(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelError)

	inner := httptest.NewRecorder()
	w := &errAfterHeaderWriter{ResponseWriter: inner}
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = req.WithContext(logctx.ContextWithRequestID(req.Context(), "list-etag-write-rid"))

	writeJSONWithETag(w, req, "tasks.list", http.StatusOK, map[string]any{"tasks": []any{}})

	line := lastLogLine(t, buf)
	if line["msg"] != "response write failed" {
		t.Fatalf("msg=%v want response write failed", line["msg"])
	}
	if line["operation"] != "tasks.list" {
		t.Fatalf("operation=%v", line["operation"])
	}
	if line["failure_stage"] != "body" {
		t.Fatalf("failure_stage=%v want body", line["failure_stage"])
	}
}

func TestWriteJSONWithETag_encodeFailure_logs(t *testing.T) {
	buf := captureHandlerLogs(t, slog.LevelError)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = req.WithContext(logctx.ContextWithRequestID(req.Context(), "list-etag-encode-rid"))

	writeJSONWithETag(rr, req, "tasks.list", http.StatusOK, map[string]any{"bad": make(chan int)})

	line := lastLogLine(t, buf)
	if line["msg"] != "response encode failed" {
		t.Fatalf("msg=%v want response encode failed", line["msg"])
	}
	if line["failure_stage"] != "response_encode" {
		t.Fatalf("failure_stage=%v want response_encode", line["failure_stage"])
	}
	if line["operation"] != "tasks.list" {
		t.Fatalf("operation=%v", line["operation"])
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d want 500", rr.Code)
	}
}
