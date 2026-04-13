package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

func TestHTTP_idempotency_post_second_replays_from_cache(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(WithIdempotency(NewHandler(st, NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	const body = `{"title":"idem-cache","priority":"medium"}`
	key := "idem-" + uuid.NewString()

	do := func() *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res1 := do()
	b1, err := io.ReadAll(res1.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := res1.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first status %d %s", res1.StatusCode, b1)
	}

	res2 := do()
	b2, err := io.ReadAll(res2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := res2.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if res2.StatusCode != http.StatusCreated {
		t.Fatalf("second status %d %s", res2.StatusCode, b2)
	}
	if string(b1) != string(b2) {
		t.Fatalf("body mismatch:\n%s\nvs\n%s", b1, b2)
	}

	var tree domain.Task
	if err := json.Unmarshal(b1, &tree); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	roots, _, err := st.ListRootForest(ctx, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, n := range roots {
		if n.Task.Title == "idem-cache" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("want 1 root task titled idem-cache, got %d", count)
	}
}

func TestHTTP_idempotency_disabled_allows_duplicate_post(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "0")

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(WithIdempotency(NewHandler(st, NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	const body = `{"title":"idem-off","priority":"medium"}`
	key := "idem-off-" + uuid.NewString()

	do := func() int {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if _, err := io.Copy(io.Discard, res.Body); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}
	first := do()
	second := do()
	if first != http.StatusCreated || second != http.StatusCreated {
		t.Fatal("expected two 201 responses")
	}

	ctx := t.Context()
	roots, _, err := st.ListRootForest(ctx, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, n := range roots {
		if n.Task.Title == "idem-off" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("want 2 tasks, got %d", count)
	}
}

func TestHTTP_idempotency_different_body_same_key_creates_two(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(WithIdempotency(NewHandler(st, NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	key := "idem-body-" + uuid.NewString()

	post := func(title string) {
		t.Helper()
		body := `{"title":"` + title + `","priority":"medium"}`
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("status %d %s", res.StatusCode, b)
		}
	}
	post("a")
	post("b")

	ctx := t.Context()
	roots, _, err := st.ListRootForest(ctx, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	titles := make(map[string]int)
	for _, n := range roots {
		titles[n.Task.Title]++
	}
	if titles["a"] != 1 || titles["b"] != 1 {
		t.Fatalf("titles: %v", titles)
	}
}

func TestHTTP_idempotency_concurrent_post_single_row(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(WithIdempotency(NewHandler(st, NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	const body = `{"title":"idem-concurrent","priority":"medium"}`
	key := "idem-conc-" + uuid.NewString()

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
			if err != nil {
				t.Error(err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", key)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := io.Copy(io.Discard, res.Body); err != nil {
				t.Error(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusCreated {
				t.Errorf("status %d", res.StatusCode)
			}
		}()
	}
	wg.Wait()

	ctx := t.Context()
	roots, _, err := st.ListRootForest(ctx, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, n := range roots {
		if n.Task.Title == "idem-concurrent" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("want 1 task, got %d", count)
	}
}

func TestWithAccessLog_idempotencyKeyTooLong_logIncludesRequestID(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(slog.New(WrapSlogHandlerWithLogSequence(base, &processSeq)))

	db := tasktestdb.OpenSQLite(t)
	h := WithAccessLog(WithIdempotency(NewHandler(store.NewStore(db), NewSSEHub(), nil)))

	longKey := strings.Repeat("k", maxIdempotencyKeyLen+1)
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":"x","priority":"medium"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", longKey)
	req.Header.Set("X-Request-ID", "rid-idem-key-long")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var warnLine map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["msg"] == "idempotency key too long" {
			warnLine = m
			break
		}
	}
	if warnLine == nil {
		t.Fatalf("no warn log in %q", buf.String())
	}
	if warnLine["request_id"] != "rid-idem-key-long" {
		t.Fatalf("request_id: %v", warnLine["request_id"])
	}
}

func TestHTTP_idempotency_rejects_overlength_key(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	srv := httptest.NewServer(WithIdempotency(NewHandler(store.NewStore(db), NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	longKey := strings.Repeat("k", maxIdempotencyKeyLen+1)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(`{"title":"idem-long","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", longKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var out jsonErrorBody
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "idempotency key too long" {
		t.Fatalf("error %q", out.Error)
	}
}

func TestHTTP_idempotency_accepts_boundary_key_length(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	srv := httptest.NewServer(WithIdempotency(NewHandler(st, NewSSEHub(), nil)))
	t.Cleanup(srv.Close)

	key := strings.Repeat("k", maxIdempotencyKeyLen)
	body := `{"title":"idem-boundary","priority":"medium"}`

	do := func() (int, string) {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, string(b)
	}

	status1, body1 := do()
	status2, body2 := do()
	if status1 != http.StatusCreated || status2 != http.StatusCreated {
		t.Fatalf("statuses %d/%d", status1, status2)
	}
	if body1 != body2 {
		t.Fatalf("expected replay with boundary key")
	}
}

func TestHTTP_idempotency_rejects_unknown_content_length(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	h := WithIdempotency(NewHandler(store.NewStore(db), NewSSEHub(), nil))

	req := httptest.NewRequest(http.MethodPost, "/tasks", io.NopCloser(strings.NewReader(`{"title":"idem-unknown","priority":"medium"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-unknown-len")
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out jsonErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "idempotency requires known content length" {
		t.Fatalf("error %q", out.Error)
	}
}

func TestHTTP_idempotency_rejects_large_content_length(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := tasktestdb.OpenSQLite(t)
	h := WithIdempotency(NewHandler(store.NewStore(db), NewSSEHub(), nil))

	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":"idem-large","priority":"medium"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-large-len")
	req.ContentLength = maxIdempotencyBodySize + 1
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out jsonErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "request body too large for idempotency" {
		t.Fatalf("error %q", out.Error)
	}
}
