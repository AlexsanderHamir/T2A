package handler

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestSSEHub_Publish_assignsMonotonicIDs pins the contract that every
// successful Publish gets the next strictly-increasing event id, even
// across goroutines. A future refactor that buckets ids per-event-type
// (or worse, hands out non-monotonic ids during a burst) would break
// Last-Event-ID resume — the client would replay the same frame twice
// or skip past frames that arrived out of order.
func TestSSEHub_Publish_assignsMonotonicIDs(t *testing.T) {
	h := NewSSEHubWith(SSEHubOptions{RingSize: 16, SubscriberBuffer: 16})
	const n = 50
	for i := 0; i < n; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: fmt.Sprintf("t-%d", i)})
	}
	if got, want := h.LastEventID(), uint64(n); got != want {
		t.Fatalf("LastEventID=%d want %d", got, want)
	}
}

// TestSSEHub_Publish_replayFromLastEventID verifies the ring-buffer
// replay path used by reconnecting EventSource clients. After 5
// publishes a fresh subscriber requesting Last-Event-ID=2 receives
// exactly events 3, 4, 5 in publish order — no gap directive, no
// stale frames mixed in. This is the foundation of "lossless SSE":
// a client whose connection blipped at id=2 reconnects, hands its
// last seen id back, and the hub replays the missing tail.
func TestSSEHub_Publish_replayFromLastEventID(t *testing.T) {
	h := NewSSEHubWith(SSEHubOptions{RingSize: 16, SubscriberBuffer: 16})
	for i := 1; i <= 5; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: fmt.Sprintf("t-%d", i)})
	}

	sub, replay, hadGap, cancel := h.subscribe(2)
	defer cancel()

	if hadGap {
		t.Fatalf("expected no gap when sinceID=2 is inside the 5-event ring")
	}
	if got := len(replay); got != 3 {
		t.Fatalf("replay length=%d want 3 (events 3,4,5)", got)
	}
	if replay[0].id != 3 || replay[1].id != 4 || replay[2].id != 5 {
		t.Fatalf("replay ids=[%d %d %d] want [3 4 5]", replay[0].id, replay[1].id, replay[2].id)
	}

	// The new subscriber's live channel must NOT also receive the
	// replayed events — they're delivered exactly once, via the
	// snapshot return value, so the writer can flush them in order
	// before entering the heartbeat/live select.
	select {
	case ev := <-sub.ch:
		t.Fatalf("subscriber got unexpected live event during replay: id=%d line=%s", ev.id, ev.line)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestSSEHub_Publish_gapDetectionForOldLastEventID verifies that a
// reconnecting client whose Last-Event-ID is older than the oldest
// retained ring entry triggers `hadGap=true`, which the HTTP handler
// translates into a single `{"type":"resync"}` directive. Without
// this branch the client would silently miss every event between
// "their last id" and "oldest still in the ring" — exactly the
// loss-mode the Phase 2 work is closing.
func TestSSEHub_Publish_gapDetectionForOldLastEventID(t *testing.T) {
	h := NewSSEHubWith(SSEHubOptions{RingSize: 4, SubscriberBuffer: 16})
	// 6 publishes into a 4-entry ring → oldest retained id is 3,
	// ids 1 and 2 are evicted.
	for i := 1; i <= 6; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: fmt.Sprintf("t-%d", i)})
	}

	_, _, hadGap, cancel := h.subscribe(1) // client says it last saw id=1
	defer cancel()
	if !hadGap {
		t.Fatalf("expected hadGap=true (sinceID=1 is older than oldest retained id=3)")
	}

	_, _, hadGapInside, cancel2 := h.subscribe(3) // client says it last saw id=3 (still in ring)
	defer cancel2()
	if hadGapInside {
		t.Fatalf("expected hadGap=false (sinceID=3 is the oldest retained id)")
	}
}

// TestSSEHub_Publish_coalescesIdenticalFrames pins the 50ms dedup
// window for `{type,id}`-identical frames. A burst of 10 identical
// `task_updated:foo` publishes inside 50ms collapses to ONE wire
// frame; the coalesced counter records the other 9 drops. Cycle
// frames carry a distinct cycle_id and are intentionally NOT
// coalesced — they're tested in the next subtest.
func TestSSEHub_Publish_coalescesIdenticalFrames(t *testing.T) {
	c := middleware.SSECoalescedCounter()
	base := testutil.ToFloat64(c)

	h := NewSSEHubWith(SSEHubOptions{
		RingSize:         16,
		SubscriberBuffer: 32,
		CoalesceWindow:   50 * time.Millisecond,
	})
	ch, cancel := h.Subscribe()
	defer cancel()

	for i := 0; i < 10; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "foo"})
	}

	// Drain everything that landed within 100ms — only the first
	// publish should have made it through; the other 9 collapsed
	// before fanout.
	delivered := 0
	timeout := time.After(100 * time.Millisecond)
drain:
	for {
		select {
		case <-ch:
			delivered++
		case <-timeout:
			break drain
		}
	}
	if delivered != 1 {
		t.Fatalf("delivered=%d want 1 (other 9 should coalesce)", delivered)
	}
	if got, want := testutil.ToFloat64(c), base+9; got != want {
		t.Fatalf("coalesced counter=%v want %v", got, want)
	}
}

// TestSSEHub_Publish_doesNotCoalesceCycleFrames pins the
// "cycle frames are informationally distinct" rule documented in the
// hub's coalesceKey: each cycle phase transition carries a different
// cycle_id, so the SPA needs every frame to refresh the right slot
// on the task detail page even when several land inside the 50ms
// window.
func TestSSEHub_Publish_doesNotCoalesceCycleFrames(t *testing.T) {
	h := NewSSEHubWith(SSEHubOptions{
		RingSize:         16,
		SubscriberBuffer: 32,
		CoalesceWindow:   50 * time.Millisecond,
	})
	ch, cancel := h.Subscribe()
	defer cancel()

	h.Publish(TaskChangeEvent{Type: TaskCycleChanged, ID: "task-1", CycleID: "c-1"})
	h.Publish(TaskChangeEvent{Type: TaskCycleChanged, ID: "task-1", CycleID: "c-1"})
	h.Publish(TaskChangeEvent{Type: TaskCycleChanged, ID: "task-1", CycleID: "c-1"})

	delivered := 0
	timeout := time.After(100 * time.Millisecond)
drain:
	for {
		select {
		case <-ch:
			delivered++
		case <-timeout:
			break drain
		}
	}
	if delivered != 3 {
		t.Fatalf("delivered=%d want 3 (cycle frames must NOT coalesce)", delivered)
	}
}

// TestSSEHub_Publish_evictsSlowConsumer pins the
// "overflow → evict + resync" backpressure contract. A subscriber
// whose channel fills up is removed from the registration set and
// its cancel channel is closed (the writer goroutine then sends a
// resync directive on the wire and shuts the HTTP stream down).
// This is loss-free under Last-Event-ID resume: the client
// reconnects with its last-seen id and replays from the ring.
func TestSSEHub_Publish_evictsSlowConsumer(t *testing.T) {
	c := middleware.SSESubscriberEvictionsCounter()
	base := testutil.ToFloat64(c)

	h := NewSSEHubWith(SSEHubOptions{
		RingSize:         128,
		SubscriberBuffer: 4,
	})
	sub, _, _, cancel := h.subscribe(0)
	defer cancel()

	// First 4 publishes fill the per-subscriber buffer (no overflow).
	for i := 0; i < 4; i++ {
		h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: fmt.Sprintf("t-%d", i)})
	}
	// Next publish overflows → eviction.
	h.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "overflow"})

	select {
	case <-sub.cancel:
		// Expected: the hub closed our cancel channel as part of eviction.
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected sub.cancel to be closed after overflow")
	}
	if got, want := testutil.ToFloat64(c), base+1; got != want {
		t.Fatalf("eviction counter=%v want %v", got, want)
	}
}

// TestHTTP_SSE_emitsIDLineForEventSourceResume verifies the on-the-wire
// frame shape every event ships with, so the browser EventSource
// captures `id: N` as Last-Event-ID for reconnect. Older deployments
// shipped only `data: ...` — a client whose connection blipped would
// reconnect with no Last-Event-ID header and silently miss every
// in-flight event. The plan calls this out as the critical wire
// contract behind lossless SSE.
func TestHTTP_SSE_emitsIDLineForEventSourceResume(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	h := NewHandler(store.NewStore(db), NewSSEHub(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	streamReady := make(chan struct{})
	gotID := make(chan uint64, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer res.Body.Close()

		br := bufio.NewReader(res.Body)
		_, _ = br.ReadString('\n') // retry: 3000
		_, _ = br.ReadString('\n') // blank line
		close(streamReady)

		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			s := strings.TrimSpace(line)
			if !strings.HasPrefix(s, "id:") {
				continue
			}
			idStr := strings.TrimSpace(strings.TrimPrefix(s, "id:"))
			n, perr := strconv.ParseUint(idStr, 10, 64)
			if perr != nil {
				t.Errorf("invalid id line %q", s)
				return
			}
			gotID <- n
			return
		}
	}()

	<-streamReady
	res, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"sse","priority":"medium"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	select {
	case id := <-gotID:
		if id == 0 {
			t.Fatalf("event id must be > 0 (monotonic counter starts at 1), got %d", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for id: line on the wire")
	}
}

// TestHTTP_SSE_replaysOnReconnectWithLastEventID is the end-to-end
// proof that the wire contract works as designed. We publish 3
// events into the hub before any client connects, then connect with
// `Last-Event-ID: 0` and assert the replay tail arrives in order.
// Without this test a future refactor that wired Last-Event-ID
// only on the in-memory subscribe path (skipping the HTTP header
// parser) would silently break browser reconnects.
func TestHTTP_SSE_replaysOnReconnectWithLastEventID(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	hub := NewSSEHubWith(SSEHubOptions{RingSize: 16, SubscriberBuffer: 32})
	h := NewHandler(store.NewStore(db), hub, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "first"})  // id=1
	hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "second"}) // id=2
	hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: "third"})  // id=3

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	// Simulate an EventSource reconnect after seeing id=1 — the
	// browser would set Last-Event-ID to the last id it captured.
	// We expect ids 2 and 3 to replay before the live tail loop.
	req.Header.Set("Last-Event-ID", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	br := bufio.NewReader(res.Body)
	var ids []string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(ids) < 2 {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "data:") && strings.Contains(s, "task_updated") {
			ids = append(ids, s)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("replay: got %d events, want 2 (lines=%v)", len(ids), ids)
	}
	if !strings.Contains(ids[0], `"id":"second"`) ||
		!strings.Contains(ids[1], `"id":"third"`) {
		t.Fatalf("replay order wrong (want second,third): %v", ids)
	}
}

// TestHTTP_SSE_emitsResyncWhenLastEventIDOutsideRing verifies the
// gap-on-reconnect path: a client whose Last-Event-ID is older than
// the oldest retained ring entry receives one `data: {"type":"resync"}`
// directive. The SPA's useTaskEventStream handler then drops every
// cache and refetches from REST — the documented escape hatch when
// the in-memory ring can't bridge the gap.
func TestHTTP_SSE_emitsResyncWhenLastEventIDOutsideRing(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	// 4-entry ring + 6 publishes → ids 1..2 are evicted, oldest
	// retained id = 3.
	hub := NewSSEHubWith(SSEHubOptions{RingSize: 4, SubscriberBuffer: 32})
	h := NewHandler(store.NewStore(db), hub, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	for i := 1; i <= 6; i++ {
		hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: fmt.Sprintf("t-%d", i)})
	}

	resyncBefore := testutil.ToFloat64(middleware.SSEResyncEmittedCounter())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	req.Header.Set("Last-Event-ID", "1") // outside the 4-entry window
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	br := bufio.NewReader(res.Body)
	sawResync := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, `"type":"resync"`) {
			sawResync = true
			break
		}
	}
	if !sawResync {
		t.Fatalf("expected one resync directive on the wire after gap reconnect")
	}
	if got, want := testutil.ToFloat64(middleware.SSEResyncEmittedCounter()), resyncBefore+1; got < want {
		t.Fatalf("resync counter=%v want >=%v", got, want)
	}
}

// TestHTTP_SSE_heartbeatLineKeepsConnectionAlive verifies that a
// silent stream still gets `: heartbeat` comment lines on the
// configured cadence. Browsers ignore the comment line per the SSE
// spec, but reverse proxies (and corporate VPN gateways) typically
// idle-kill TCP connections after 30-60s with no traffic — without
// the heartbeat the client would see a forced disconnect every
// minute even when the server is healthy.
func TestHTTP_SSE_heartbeatLineKeepsConnectionAlive(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	hub := NewSSEHubWith(SSEHubOptions{
		RingSize:         16,
		SubscriberBuffer: 32,
		HeartbeatPeriod:  50 * time.Millisecond,
	})
	h := NewHandler(store.NewStore(db), hub, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	br := bufio.NewReader(res.Body)
	saw := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(line), ":") {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("expected at least one `: heartbeat` line within 2s (period=50ms)")
	}
}

// TestSSEHub_Publish_concurrentSafetyUnderLoad is the race-detector
// regression for the new ring + coalesce + subscriber paths. With
// `-race`, this would catch any unsynchronized write to the ring
// buffer, the lastEmitted map, or the subs map. Numbers are tuned to
// keep the test fast while still exercising every concurrent code
// path.
func TestSSEHub_Publish_concurrentSafetyUnderLoad(t *testing.T) {
	h := NewSSEHubWith(SSEHubOptions{
		RingSize:         128,
		SubscriberBuffer: 64,
		CoalesceWindow:   1 * time.Millisecond,
	})

	var wg sync.WaitGroup
	const subs = 8
	const publishersPerSub = 4
	const eventsPerPublisher = 200

	for i := 0; i < subs; i++ {
		ch, cancel := h.Subscribe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			deadline := time.After(2 * time.Second)
			for {
				select {
				case <-ch:
				case <-deadline:
					return
				}
			}
		}()
	}

	var pub sync.WaitGroup
	for p := 0; p < publishersPerSub; p++ {
		pub.Add(1)
		go func(idx int) {
			defer pub.Done()
			for i := 0; i < eventsPerPublisher; i++ {
				h.Publish(TaskChangeEvent{
					Type: TaskUpdated,
					ID:   fmt.Sprintf("p%d-%d", idx, i),
				})
			}
		}(p)
	}
	pub.Wait()
	wg.Wait()

	if h.LastEventID() == 0 {
		t.Fatalf("LastEventID should advance under concurrent publish")
	}
}

