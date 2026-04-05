package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

func TestHTTP_idempotency_post_second_replays_from_cache(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")

	db := testdb.OpenSQLite(t)
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

	db := testdb.OpenSQLite(t)
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
	if do() != http.StatusCreated || do() != http.StatusCreated {
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

	db := testdb.OpenSQLite(t)
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

	db := testdb.OpenSQLite(t)
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

func TestIdempotencyTTLConfigured(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "")
	if idempotencyTTLConfigured() != defaultIdempotencyTTL || IdempotencyTTL() != defaultIdempotencyTTL {
		t.Fatalf("default ttl")
	}
	t.Setenv("T2A_IDEMPOTENCY_TTL", "0")
	if idempotencyTTLConfigured() != 0 || IdempotencyTTL() != 0 {
		t.Fatalf("zero")
	}
	t.Setenv("T2A_IDEMPOTENCY_TTL", "30m")
	if got := idempotencyTTLConfigured(); got != 30*time.Minute || IdempotencyTTL() != got {
		t.Fatalf("30m: got %v", got)
	}
	t.Setenv("T2A_IDEMPOTENCY_TTL", "not-a-duration")
	if idempotencyTTLConfigured() != defaultIdempotencyTTL || IdempotencyTTL() != defaultIdempotencyTTL {
		t.Fatalf("invalid falls back")
	}
}
