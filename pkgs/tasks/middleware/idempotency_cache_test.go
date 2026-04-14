package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestIdempotencyCache_set_enforces_entry_limit(t *testing.T) {
	t.Cleanup(ClearIdempotencyStateForTest)
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

func TestIdempotencyCache_set_evictionsIncrementPrometheusCounter(t *testing.T) {
	t.Cleanup(ClearIdempotencyStateForTest)
	t.Setenv("T2A_IDEMPOTENCY_MAX_ENTRIES", "2")
	t.Setenv("T2A_IDEMPOTENCY_MAX_BYTES", "0")

	before := testutil.ToFloat64(taskapiHTTPIdempotencyCacheEvictionsTotal)
	now := time.Now()
	idempCache.set(context.Background(), "k1", idempotencyCaptured{status: http.StatusCreated, body: []byte("a")}, now.Add(time.Hour))
	idempCache.set(context.Background(), "k2", idempotencyCaptured{status: http.StatusCreated, body: []byte("b")}, now.Add(time.Hour))
	idempCache.set(context.Background(), "k3", idempotencyCaptured{status: http.StatusCreated, body: []byte("c")}, now.Add(time.Hour))

	after := testutil.ToFloat64(taskapiHTTPIdempotencyCacheEvictionsTotal)
	if after-before < 1 {
		t.Fatalf("expected eviction counter to increase, before=%v after=%v", before, after)
	}
}

func TestIdempotencyCache_set_enforces_byte_limit(t *testing.T) {
	t.Cleanup(ClearIdempotencyStateForTest)
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
