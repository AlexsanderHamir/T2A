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
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/logctx"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/google/uuid"
)

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

func TestIdempotencyCacheLimitsConfigured(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "")
	maxEntries, maxBytes := IdempotencyCacheLimits()
	if maxEntries != defaultIdempotencyMaxEntries || maxBytes != defaultIdempotencyMaxBytes {
		t.Fatalf("defaults got entries=%d bytes=%d", maxEntries, maxBytes)
	}

	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "128")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "262144")
	maxEntries, maxBytes = IdempotencyCacheLimits()
	if maxEntries != 128 || maxBytes != 262144 {
		t.Fatalf("configured got entries=%d bytes=%d", maxEntries, maxBytes)
	}

	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "-1")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "nope")
	maxEntries, maxBytes = IdempotencyCacheLimits()
	if maxEntries != defaultIdempotencyMaxEntries || maxBytes != defaultIdempotencyMaxBytes {
		t.Fatalf("invalid fallback got entries=%d bytes=%d", maxEntries, maxBytes)
	}
}

func TestIdempotencyCache_set_enforces_entry_limit(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "2")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "0")

	now := time.Now()
	idempCache.set(context.Background(), "k1", idempotencyCaptured{status: http.StatusCreated, body: []byte("a")}, now.Add(time.Hour))
	idempCache.set(context.Background(), "k2", idempotencyCaptured{status: http.StatusCreated, body: []byte("b")}, now.Add(time.Hour))
	idempCache.set(context.Background(), "k3", idempotencyCaptured{status: http.StatusCreated, body: []byte("c")}, now.Add(time.Hour))

	if _, ok := idempCache.get("k1"); ok {
		t.Fatalf("oldest key should be evicted")
	}
	if _, ok := idempCache.get("k2"); !ok {
		t.Fatalf("k2 should remain")
	}
	if _, ok := idempCache.get("k3"); !ok {
		t.Fatalf("k3 should remain")
	}
}

func TestIdempotencyCache_set_enforces_byte_limit(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "0")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "5")

	now := time.Now()
	idempCache.set(context.Background(), "k1", idempotencyCaptured{status: http.StatusCreated, body: []byte("111")}, now.Add(time.Hour))
	idempCache.set(context.Background(), "k2", idempotencyCaptured{status: http.StatusCreated, body: []byte("22")}, now.Add(time.Hour))
	idempCache.set(context.Background(), "k3", idempotencyCaptured{status: http.StatusCreated, body: []byte("3")}, now.Add(time.Hour))

	if _, ok := idempCache.get("k1"); ok {
		t.Fatalf("k1 should be evicted to satisfy byte cap")
	}
	if _, ok := idempCache.get("k2"); !ok {
		t.Fatalf("k2 should remain")
	}
	if _, ok := idempCache.get("k3"); !ok {
		t.Fatalf("k3 should remain")
	}
}

func TestWithAccessLog_idempotencyCacheEviction_logIncludesRequestID(t *testing.T) {
	t.Cleanup(clearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_TTL", "1h")
	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "2")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "0")

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	var processSeq atomic.Uint64
	base := logctx.WrapSlogHandlerWithRequestContext(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(slog.New(logctx.WrapSlogHandlerWithLogSequence(base, &processSeq)))

	db := tasktestdb.OpenSQLite(t)
	srv := httptest.NewServer(WithAccessLog(WithIdempotency(NewHandler(store.NewStore(db), NewSSEHub(), nil))))
	t.Cleanup(srv.Close)

	const rid = "rid-idem-cache-evict"
	post := func(key, title string) {
		t.Helper()
		body := `{"title":"` + title + `","priority":"medium"}`
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/tasks", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("X-Request-ID", rid)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("post key %s: status %d", key, res.StatusCode)
		}
	}

	post("idem-evict-"+uuid.NewString(), "e1")
	post("idem-evict-"+uuid.NewString(), "e2")
	post("idem-evict-"+uuid.NewString(), "e3")

	var evictLine map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["msg"] == "idempotency cache evicted entries" {
			evictLine = m
			break
		}
	}
	if evictLine == nil {
		t.Fatalf("no eviction log in: %q", buf.String())
	}
	if evictLine["request_id"] != rid {
		t.Fatalf("request_id: %v", evictLine["request_id"])
	}
}
