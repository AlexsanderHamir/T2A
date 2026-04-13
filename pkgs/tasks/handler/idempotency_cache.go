package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
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

var idempCache = &idempotencyCache{items: make(map[string]idempotencyEntry)}

func (c *idempotencyCache) get(key string) (idempotencyCaptured, bool) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyCache.get")
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

func (c *idempotencyCache) set(ctx context.Context, key string, cap idempotencyCaptured, until time.Time) {
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
		logCtx := ctx
		if logCtx == nil {
			logCtx = context.Background()
		}
		slog.Log(logCtx, slog.LevelWarn, "idempotency cache evicted entries",
			"cmd", httpLogCmd, "operation", "handler.idempotency",
			"evicted", evicted, "entries", len(c.items), "bytes", c.totalBytes,
			"max_entries", maxEntries, "max_bytes", maxBytes)
	}
}

func (c *idempotencyCache) pruneLocked(now time.Time) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyCache.pruneLocked")
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
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.idempotencyCache.enforceLimitsLocked")
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
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.clearIdempotencyStateForTest")
	idempCache.mu.Lock()
	idempCache.items = make(map[string]idempotencyEntry)
	idempCache.sets = 0
	idempCache.nextSeq = 0
	idempCache.totalBytes = 0
	idempCache.mu.Unlock()
}
