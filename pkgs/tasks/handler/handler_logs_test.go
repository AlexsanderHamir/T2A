package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func newLogsTestServer(t *testing.T, logDir string) *httptest.Server {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil, WithLogDirectory(logDir))
	return httptest.NewServer(h)
}

func writeLogFile(t *testing.T, dir, name, body string, mod time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestHTTP_logsList_returnsTaskAPILogsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	older := time.Date(2026, 4, 24, 15, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	writeLogFile(t, dir, "taskapi-2026-04-24-150000-000000001.jsonl", "{}\n", older)
	writeLogFile(t, dir, "taskapi-2026-04-24-160000-000000001.jsonl", "{}\n", newer)
	writeLogFile(t, dir, "notes.jsonl", "{}\n", newer.Add(time.Hour))

	srv := newLogsTestServer(t, dir)
	defer srv.Close()

	raw, _ := mustGetJSON(t, srv.URL, "/logs")
	var got listLogsResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	if len(got.Logs) != 2 {
		t.Fatalf("logs len=%d want 2 body=%s", len(got.Logs), raw)
	}
	if got.Logs[0].Name != "taskapi-2026-04-24-160000-000000001.jsonl" {
		t.Fatalf("first log = %q", got.Logs[0].Name)
	}
}

func TestHTTP_logEntries_filtersAndPagesJSONL(t *testing.T) {
	dir := t.TempDir()
	name := "taskapi-2026-04-24-160000-000000001.jsonl"
	writeLogFile(t, dir, name, strings.Join([]string{
		`{"time":"2026-04-24T16:00:00Z","level":"INFO","msg":"one","operation":"task.create","request_id":"r1","log_seq":1}`,
		`{"time":"2026-04-24T16:00:01Z","level":"WARN","msg":"two","operation":"task.patch","request_id":"r2","log_seq":2}`,
		`{"time":"2026-04-24T16:00:02Z","level":"ERROR","msg":"three","operation":"task.patch","request_id":"r2","log_seq":3}`,
		`not-json`,
	}, "\n")+"\n", time.Now())

	srv := newLogsTestServer(t, dir)
	defer srv.Close()

	raw, _ := mustGetJSON(t, srv.URL, "/logs/"+name+"?operation=task.patch&limit=1")
	var page logEntriesResponse
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("decode page: %v body=%s", err, raw)
	}
	if len(page.Entries) != 1 || page.Entries[0].Line != 2 || !page.HasMore {
		t.Fatalf("unexpected first page: %+v body=%s", page, raw)
	}

	raw, _ = mustGetJSON(t, srv.URL, "/logs/"+name+"?operation=task.patch&limit=1&offset=2")
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("decode second page: %v body=%s", err, raw)
	}
	if len(page.Entries) != 1 || page.Entries[0].Line != 3 {
		t.Fatalf("unexpected second page: %+v body=%s", page, raw)
	}
}

func TestHTTP_logEntries_rejectsTraversal(t *testing.T) {
	srv := newLogsTestServer(t, t.TempDir())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/logs/..%2Fsecret.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusNotFound {
		t.Fatalf("GET traversal status=%d want 400 or mux 404", res.StatusCode)
	}
}
