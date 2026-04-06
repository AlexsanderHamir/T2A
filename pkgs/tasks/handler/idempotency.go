package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/singleflight"
)

const (
	idempotencyHeader            = "Idempotency-Key"
	maxIdempotencyKeyLen         = 128
	maxIdempotencyBodySize       = 1 << 20 // 1 MiB
	defaultIdempotencyTTL        = 24 * time.Hour
	idempotencyPruneMod          = 256
	defaultIdempotencyMaxEntries = 2048
	defaultIdempotencyMaxBytes   = 8 << 20 // 8 MiB
)

var (
	taskapiHTTPIdempotentReplayTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "taskapi",
		Name:      "http_idempotent_replay_total",
		Help:      "Responses served from idempotency cache (same key, method, path, and body hash as first successful response).",
	})
)

type idempotencyCaptured struct {
	status  int
	headers http.Header
	body    []byte
}

type idempotencyEntry struct {
	until time.Time
	cap   idempotencyCaptured
	size  int
	seq   uint64
}

type idempotencyCache struct {
	mu         sync.Mutex
	items      map[string]idempotencyEntry
	sets       uint64
	nextSeq    uint64
	totalBytes int
}

var (
	idempCache = &idempotencyCache{items: make(map[string]idempotencyEntry)}
	idempSF    singleflight.Group
)

func idempotencyTTLConfigured() time.Duration {
	s := strings.TrimSpace(os.Getenv("T2A_IDEMPOTENCY_TTL"))
	if s == "" {
		return defaultIdempotencyTTL
	}
	if s == "0" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return defaultIdempotencyTTL
	}
	return d
}

func idempotencyMaxEntriesConfigured() int {
	s := strings.TrimSpace(os.Getenv("T2A_IDEMPOTENCY_MAX_ENTRIES"))
	if s == "" {
		return defaultIdempotencyMaxEntries
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultIdempotencyMaxEntries
	}
	return n
}

func idempotencyMaxBytesConfigured() int {
	s := strings.TrimSpace(os.Getenv("T2A_IDEMPOTENCY_MAX_BYTES"))
	if s == "" {
		return defaultIdempotencyMaxBytes
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultIdempotencyMaxBytes
	}
	return n
}

// IdempotencyTTL returns the effective in-process idempotency cache TTL from
// T2A_IDEMPOTENCY_TTL (same as WithIdempotency): default 24h, 0 disables caching.
func IdempotencyTTL() time.Duration {
	return idempotencyTTLConfigured()
}

// IdempotencyCacheLimits returns effective in-process idempotency cache limits.
// 0 means disabled for the respective bound.
func IdempotencyCacheLimits() (maxEntries int, maxBytes int) {
	return idempotencyMaxEntriesConfigured(), idempotencyMaxBytesConfigured()
}

func idempotencyMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func shouldCacheIdempotentStatus(code int) bool {
	switch code {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return true
	default:
		return false
	}
}

func cloneIdempotentResponseHeaders(src http.Header) http.Header {
	dst := make(http.Header)
	for _, k := range []string{"Content-Type", "X-Content-Type-Options"} {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
	return dst
}

func captureIdempotentResponse(rec *httptest.ResponseRecorder) idempotencyCaptured {
	status := rec.Code
	if status == 0 {
		status = http.StatusOK
	}
	hdr := cloneIdempotentResponseHeaders(rec.Header())
	body := rec.Body.Bytes()
	b := make([]byte, len(body))
	copy(b, body)
	return idempotencyCaptured{status: status, headers: hdr, body: b}
}

func replayIdempotentResponse(w http.ResponseWriter, cap idempotencyCaptured) {
	setAPISecurityHeaders(w)
	if v := cap.headers.Get("Content-Type"); v != "" {
		w.Header().Set("Content-Type", v)
	}
	w.WriteHeader(cap.status)
	if len(cap.body) > 0 {
		_, _ = w.Write(cap.body)
	}
}

func (c *idempotencyCache) get(key string) (idempotencyCaptured, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok || now.After(e.until) {
		if ok {
			c.totalBytes -= e.size
			if c.totalBytes < 0 {
				c.totalBytes = 0
			}
			delete(c.items, key)
		}
		return idempotencyCaptured{}, false
	}
	return e.cap, true
}

func (c *idempotencyCache) set(key string, cap idempotencyCaptured, until time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.items[key]; ok {
		c.totalBytes -= old.size
		if c.totalBytes < 0 {
			c.totalBytes = 0
		}
	}
	c.nextSeq++
	size := len(cap.body)
	c.items[key] = idempotencyEntry{until: until, cap: cap, size: size, seq: c.nextSeq}
	c.totalBytes += size
	c.sets++
	if c.sets%idempotencyPruneMod == 0 {
		c.pruneLocked(time.Now())
	}
	evicted := c.enforceLimitsLocked()
	if evicted > 0 {
		maxEntries, maxBytes := IdempotencyCacheLimits()
		slog.Warn("idempotency cache evicted entries", "cmd", httpLogCmd, "operation", "handler.idempotency",
			"evicted", evicted, "entries", len(c.items), "bytes", c.totalBytes,
			"max_entries", maxEntries, "max_bytes", maxBytes)
	}
}

func (c *idempotencyCache) pruneLocked(now time.Time) {
	for k, e := range c.items {
		if now.After(e.until) {
			c.totalBytes -= e.size
			if c.totalBytes < 0 {
				c.totalBytes = 0
			}
			delete(c.items, k)
		}
	}
}

func (c *idempotencyCache) enforceLimitsLocked() int {
	maxEntries, maxBytes := IdempotencyCacheLimits()
	if maxEntries == 0 && maxBytes == 0 {
		return 0
	}
	var evicted int
	for {
		overEntries := maxEntries > 0 && len(c.items) > maxEntries
		overBytes := maxBytes > 0 && c.totalBytes > maxBytes
		if !overEntries && !overBytes {
			return evicted
		}
		var oldestKey string
		var oldestEntry idempotencyEntry
		found := false
		for k, e := range c.items {
			if !found || e.seq < oldestEntry.seq {
				oldestKey = k
				oldestEntry = e
				found = true
			}
		}
		if !found {
			return evicted
		}
		c.totalBytes -= oldestEntry.size
		if c.totalBytes < 0 {
			c.totalBytes = 0
		}
		delete(c.items, oldestKey)
		evicted++
	}
}

// clearIdempotencyStateForTest resets in-memory idempotency state (handler package tests only).
func clearIdempotencyStateForTest() {
	idempCache.mu.Lock()
	idempCache.items = make(map[string]idempotencyEntry)
	idempCache.sets = 0
	idempCache.nextSeq = 0
	idempCache.totalBytes = 0
	idempCache.mu.Unlock()
}

// WithIdempotency deduplicates mutating requests that send a non-empty Idempotency-Key header.
// The composite key is method, URL path, trimmed key (max 128 bytes), and SHA-256 of the body for
// POST/PATCH (DELETE omits a body fingerprint). Only responses with status 200, 201, or 204 are
// cached. Concurrent identical requests share one handler execution via singleflight.
//
// POST/PATCH with unknown Content-Length (chunked), negative length, or a body larger than 1 MiB
// skip this middleware (handler runs normally; key is ignored for caching).
//
// Cache TTL comes from T2A_IDEMPOTENCY_TTL (Go duration; default 24h). Set to 0 to disable.
// The cache is in-process only and is not shared across replicas.
func WithIdempotency(h http.Handler) http.Handler {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.WithIdempotency")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ttl := idempotencyTTLConfigured()
		if ttl <= 0 || !idempotencyMutatingMethod(r.Method) {
			h.ServeHTTP(w, r)
			return
		}
		rawKey := strings.TrimSpace(r.Header.Get(idempotencyHeader))
		if rawKey == "" {
			h.ServeHTTP(w, r)
			return
		}
		key := rawKey
		if len(key) > maxIdempotencyKeyLen {
			key = key[:maxIdempotencyKeyLen]
		}

		var bodyFP string
		switch r.Method {
		case http.MethodPost, http.MethodPatch:
			if r.ContentLength < 0 || r.ContentLength > maxIdempotencyBodySize {
				h.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				slog.Warn("idempotency body read failed", "cmd", httpLogCmd, "operation", "handler.idempotency", "err", err)
				writeJSONError(w, r, "idempotency.read_body", http.StatusBadRequest, "could not read request body")
				return
			}
			_ = r.Body.Close()
			sum := sha256.Sum256(body)
			bodyFP = hex.EncodeToString(sum[:])
			r.Body = io.NopCloser(bytes.NewReader(body))
		}

		composite := r.Method + "\x00" + r.URL.Path + "\x00" + key + "\x00" + bodyFP

		if cap, ok := idempCache.get(composite); ok {
			taskapiHTTPIdempotentReplayTotal.Inc()
			replayIdempotentResponse(w, cap)
			return
		}

		v, err, _ := idempSF.Do(composite, func() (interface{}, error) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			cap := captureIdempotentResponse(rec)
			if shouldCacheIdempotentStatus(cap.status) {
				idempCache.set(composite, cap, time.Now().Add(ttl))
			}
			return cap, nil
		})
		if err != nil {
			slog.Error("idempotency singleflight error", "cmd", httpLogCmd, "operation", "handler.idempotency", "err", err)
			writeJSONError(w, r, "idempotency", http.StatusInternalServerError, "internal server error")
			return
		}
		cap := v.(idempotencyCaptured)
		replayIdempotentResponse(w, cap)
	})
}
