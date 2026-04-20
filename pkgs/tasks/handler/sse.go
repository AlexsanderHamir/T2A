package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

// TaskChangeType names SSE payload types for task lifecycle.
type TaskChangeType string

const (
	TaskCreated      TaskChangeType = "task_created"
	TaskUpdated      TaskChangeType = "task_updated"
	TaskDeleted      TaskChangeType = "task_deleted"
	TaskCycleChanged TaskChangeType = "task_cycle_changed"
	// SettingsChanged fires after PATCH /settings persists or the
	// agent worker supervisor restarts as a result of a settings change.
	// The event has no ID/CycleID; consumers refetch GET /settings to
	// pick up the new values. Documented in docs/API-SSE.md.
	SettingsChanged TaskChangeType = "settings_changed"
	// AgentRunCancelled fires after POST /settings/cancel-current-run
	// successfully cancels an in-flight runner.Run. Listeners use it
	// to flip the SPA "Cancel current run" button back to its idle
	// state without polling.
	AgentRunCancelled TaskChangeType = "agent_run_cancelled"
	// Resync is a hub-emitted directive that tells the client its
	// reconnect cursor is outside the ring buffer (or it was forcibly
	// disconnected as a slow consumer) and it should drop all caches
	// and refetch from the REST API. Wire payload: `{"type":"resync"}`
	// with no id/cycle_id. Documented in docs/API-SSE.md.
	Resync TaskChangeType = "resync"
)

// TaskChangeEvent is a minimal JSON line sent as one SSE "data:" frame.
//
// CycleID is only set for `task_cycle_changed` events so the SPA can
// invalidate just the affected cycle subtree instead of the whole task.
// It is omitted from the wire for every other type to keep existing
// payloads byte-identical to the pre-Stage-5 contract.
type TaskChangeEvent struct {
	Type    TaskChangeType `json:"type"`
	ID      string         `json:"id"`
	CycleID string         `json:"cycle_id,omitempty"`
}

// bufferedEvent is one slot in the SSEHub ring buffer used to replay
// missed frames to a reconnecting client that supplied a Last-Event-ID
// header. `id` is the monotonically increasing event id allocated at
// Publish time; `line` is the already-marshalled JSON payload (cached
// so replay does not re-marshal); `at` is the wall-clock for lag math.
type bufferedEvent struct {
	id   uint64
	line string
	at   time.Time
}

// subscriber tracks one connected /events client. The hub owns the
// channel; the streamEvents goroutine drains it. The HTTP path uses
// `ch`; the legacy in-process Subscribe() entry-point uses `legacy`
// (a string channel) so existing tests and devsim consumers don't
// have to know about the new bufferedEvent payload. A subscriber
// has exactly one of the two channels populated; the unused one is
// left nil so the publish path skips it.
type subscriber struct {
	ch     chan bufferedEvent // HTTP /events writer (replay+heartbeat aware)
	legacy chan string        // legacy in-process consumer (raw JSON line)
	cancel chan struct{}      // closed by hub when the slow-consumer eviction fires
}

// SSEHub fans out task change notifications to all connected SSE clients.
//
// The hub keeps a bounded ring buffer of recent events keyed by a
// monotonically increasing event id. The HTTP handler honors the
// browser's Last-Event-ID header on reconnect by replaying the tail of
// the ring; if the requested id is older than the oldest retained
// event the handler emits one `{"type":"resync"}` frame so the client
// drops every cache and starts over from REST. Slow consumers are
// evicted (their cancel channel is closed) instead of silently dropping
// frames — they reconnect, replay via Last-Event-ID, and either catch
// up or fall through to the same resync directive. End result: the
// /events stream is semantically lossless even when individual TCP
// connections stutter.
//
// Coalescing: identical {Type, ID} task / settings frames published
// inside the coalesceWindow collapse to one wire frame so a burst of
// supervisor reloads or duplicate task_updated events does not spam
// the fanout. Cycle frames carry a distinct cycle_id and are not
// coalesced.
type SSEHub struct {
	mu              sync.Mutex
	subs            map[*subscriber]struct{}
	ring            []bufferedEvent
	ringHead        int  // next write slot (oldest evicted on overwrite)
	ringFilled      bool // true once we've wrapped at least once
	nextID          atomic.Uint64
	coalesceWindow  time.Duration
	lastEmitted     map[string]coalesceEntry // key = "type:id" (cycle_id excluded → cycle frames never coalesce)
	subBuf          int                      // per-subscriber channel capacity
	heartbeatPeriod time.Duration
}

// coalesceEntry records the at-time of the last frame whose key matches
// the candidate key, so Publish can drop a duplicate that arrives
// within coalesceWindow.
type coalesceEntry struct {
	at time.Time
}

// SSEHubOptions tunes the hub. Zero values pick safe production
// defaults; tests override individual fields to exercise edge cases.
type SSEHubOptions struct {
	// RingSize is the number of buffered events retained for replay.
	// Each entry is ~120 bytes of JSON for typical payloads, so 1024
	// keeps memory under ~125 KB per hub. Must be > 0.
	RingSize int
	// SubscriberBuffer is the per-subscriber channel capacity. When the
	// channel fills the subscriber is evicted with a resync directive
	// rather than silently dropping frames. Must be > 0.
	SubscriberBuffer int
	// CoalesceWindow is the dedup window for {type,id}-identical frames.
	// 0 disables coalescing.
	CoalesceWindow time.Duration
	// HeartbeatPeriod is the comment-line keep-alive cadence for each
	// connected client. 0 disables heartbeats.
	HeartbeatPeriod time.Duration
}

// DefaultSSEHubOptions are the production-grade defaults documented in
// the architecture plan: 1024-event ring, 256-frame per-subscriber
// buffer, 50ms coalescing window, 15s heartbeats.
func DefaultSSEHubOptions() SSEHubOptions {
	return SSEHubOptions{
		RingSize:         1024,
		SubscriberBuffer: 256,
		CoalesceWindow:   50 * time.Millisecond,
		HeartbeatPeriod:  15 * time.Second,
	}
}

// NewSSEHub returns a hub with no subscribers wired to test-friendly
// defaults: ring + per-subscriber buffer + heartbeats are kept (they
// are loss-prevention, not behavioral changes) but coalescing is OFF
// so back-to-back distinct operations never collide on the 50ms
// dedup window in unit tests. Production wiring in
// `cmd/taskapi/run_helpers.go` opts in to the production defaults via
// `NewSSEHubWith(DefaultSSEHubOptions())`. It is safe for concurrent
// use.
func NewSSEHub() *SSEHub {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewSSEHub")
	opts := DefaultSSEHubOptions()
	opts.CoalesceWindow = 0
	return NewSSEHubWith(opts)
}

// NewSSEHubWith builds a hub with caller-supplied tuning. Invalid values
// (zero or negative) fall back to the matching DefaultSSEHubOptions
// field so callers can override one knob at a time.
func NewSSEHubWith(opts SSEHubOptions) *SSEHub {
	d := DefaultSSEHubOptions()
	if opts.RingSize <= 0 {
		opts.RingSize = d.RingSize
	}
	if opts.SubscriberBuffer <= 0 {
		opts.SubscriberBuffer = d.SubscriberBuffer
	}
	if opts.CoalesceWindow < 0 {
		opts.CoalesceWindow = 0
	}
	if opts.HeartbeatPeriod < 0 {
		opts.HeartbeatPeriod = 0
	}
	return &SSEHub{
		subs:            make(map[*subscriber]struct{}),
		ring:            make([]bufferedEvent, opts.RingSize),
		coalesceWindow:  opts.CoalesceWindow,
		lastEmitted:     make(map[string]coalesceEntry),
		subBuf:          opts.SubscriberBuffer,
		heartbeatPeriod: opts.HeartbeatPeriod,
	}
}

// subscribe registers a subscriber and replays the tail of the ring
// from sinceID+1 (exclusive). If sinceID is older than the oldest
// retained event, replay is skipped and the caller is expected to
// emit a single resync frame to the client. The returned hadGap
// flag is true when a gap was detected so the handler can do that.
//
// The cancel returned closes the subscriber registration; it is also
// invoked by the hub itself when the subscriber is evicted as a slow
// consumer.
func (h *SSEHub) subscribe(sinceID uint64) (sub *subscriber, replay []bufferedEvent, hadGap bool, cancel func()) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.SSEHub.subscribe", "since_id", sinceID)
	sub = &subscriber{
		ch:     make(chan bufferedEvent, h.subBuf),
		cancel: make(chan struct{}),
	}
	h.mu.Lock()
	if sinceID > 0 {
		replay, hadGap = h.snapshotSinceLocked(sinceID)
	}
	h.subs[sub] = struct{}{}
	n := len(h.subs)
	h.mu.Unlock()
	middleware.RecordSSESubscriberGauge(n)
	cancel = func() {
		h.mu.Lock()
		if _, ok := h.subs[sub]; ok {
			delete(h.subs, sub)
		}
		n := len(h.subs)
		h.mu.Unlock()
		middleware.RecordSSESubscriberGauge(n)
	}
	return sub, replay, hadGap, cancel
}

// snapshotSinceLocked returns ring entries with id > sinceID in publish
// order. hadGap is true when sinceID is older than the oldest retained
// event (caller should send a resync directive instead of replaying).
// Caller must hold h.mu.
func (h *SSEHub) snapshotSinceLocked(sinceID uint64) (out []bufferedEvent, hadGap bool) {
	if !h.ringFilled && h.ringHead == 0 {
		return nil, false
	}
	// Walk the ring in chronological order: oldest slot first.
	size := len(h.ring)
	start := 0
	count := h.ringHead
	if h.ringFilled {
		start = h.ringHead
		count = size
	}
	out = make([]bufferedEvent, 0, count)
	var oldestID uint64
	for i := 0; i < count; i++ {
		e := h.ring[(start+i)%size]
		if i == 0 {
			oldestID = e.id
		}
		if e.id > sinceID {
			out = append(out, e)
		}
	}
	if sinceID < oldestID-1 || (sinceID == 0 && oldestID > 1) {
		hadGap = true
	}
	return out, hadGap
}

// appendRingLocked stores a new bufferedEvent in the ring, evicting the
// oldest entry when the buffer is full. Caller must hold h.mu.
func (h *SSEHub) appendRingLocked(ev bufferedEvent) {
	h.ring[h.ringHead] = ev
	h.ringHead++
	if h.ringHead == len(h.ring) {
		h.ringHead = 0
		h.ringFilled = true
	}
}

// coalesceKey returns a canonical "type:id" string used to dedup
// identical frames inside the coalesceWindow. Cycle frames carry a
// distinct cycle id so they are *intentionally* not coalesced — each
// phase transition is informationally distinct.
func coalesceKey(ev TaskChangeEvent) string {
	if ev.Type == TaskCycleChanged {
		return ""
	}
	return string(ev.Type) + ":" + ev.ID
}

// Publish delivers one JSON-encoded event to all current subscribers.
//
// The publish path: (a) coalesce identical {type,id} frames inside the
// configured window, (b) allocate a monotonic event id and append to
// the ring buffer, (c) non-blocking send to every subscriber, (d)
// evict any subscriber whose channel is full (they reconnect and
// replay via Last-Event-ID, falling through to a resync directive if
// the gap exceeds the ring window).
func (h *SSEHub) Publish(ev TaskChangeEvent) {
	if h == nil {
		return
	}
	now := time.Now()
	if h.coalesceWindow > 0 {
		key := coalesceKey(ev)
		if key != "" {
			h.mu.Lock()
			last, seen := h.lastEmitted[key]
			if seen && now.Sub(last.at) < h.coalesceWindow {
				h.mu.Unlock()
				middleware.RecordSSECoalesced(1)
				slog.Debug("sse coalesce drop",
					"cmd", calltrace.LogCmd, "operation", "tasks.sse.publish",
					"event_type", ev.Type, "task_id", ev.ID,
					"window_ms", h.coalesceWindow.Milliseconds())
				return
			}
			h.lastEmitted[key] = coalesceEntry{at: now}
			h.mu.Unlock()
		}
	}
	b, err := json.Marshal(ev)
	if err != nil {
		slog.Error("sse publish marshal failed", "cmd", calltrace.LogCmd, "operation", "tasks.sse.publish", "err", err)
		return
	}
	id := h.nextID.Add(1)
	be := bufferedEvent{id: id, line: string(b), at: now}

	h.mu.Lock()
	h.appendRingLocked(be)
	out := make([]*subscriber, 0, len(h.subs))
	for s := range h.subs {
		out = append(out, s)
	}
	h.mu.Unlock()

	dropped := 0
	evicted := 0
	for _, s := range out {
		// Each subscriber has exactly one of (ch, legacy) populated.
		// Legacy in-process consumers (Subscribe()) get the raw JSON
		// line; HTTP /events subscribers get the typed event so the
		// writer can render the `id:` line for EventSource resume.
		if s.legacy != nil {
			select {
			case s.legacy <- be.line:
			default:
				dropped++
			}
			continue
		}
		select {
		case s.ch <- be:
		default:
			// Slow consumer: evict instead of silently dropping.
			// Eviction closes cancel so the writer goroutine sends
			// a resync frame and shuts the HTTP stream down.
			h.evictSubscriber(s)
			evicted++
			dropped++
		}
	}
	if dropped > 0 {
		middleware.RecordSSEDroppedFrames(dropped)
	}
	if evicted > 0 {
		middleware.RecordSSESubscriberEvictions(evicted)
		slog.Warn("sse evicted slow subscribers",
			"cmd", calltrace.LogCmd, "operation", "tasks.sse.publish",
			"event_type", ev.Type, "task_id", ev.ID,
			"event_id", id, "evicted", evicted, "remaining", len(out)-evicted)
	}
	if len(out) > 0 {
		slog.Debug("sse fanout", "cmd", calltrace.LogCmd, "operation", "tasks.sse.publish",
			"event_type", ev.Type, "task_id", ev.ID, "event_id", id,
			"subscribers", len(out), "dropped", dropped, "evicted", evicted)
	}
}

// evictSubscriber removes the subscriber from the registration set and
// signals the writer goroutine via cancel. The writer is responsible
// for draining its channel, sending a `{"type":"resync"}` directive,
// and closing the HTTP stream so the client reconnects.
func (h *SSEHub) evictSubscriber(s *subscriber) {
	h.mu.Lock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		// Closing cancel exactly once — guarded by the membership check.
		close(s.cancel)
	}
	n := len(h.subs)
	h.mu.Unlock()
	middleware.RecordSSESubscriberGauge(n)
}

// LastEventID returns the highest event id allocated by the hub. Tests
// use it to assert publish ordering. Returns 0 if nothing was published.
func (h *SSEHub) LastEventID() uint64 {
	if h == nil {
		return 0
	}
	return h.nextID.Load()
}

// parseLastEventIDHeader parses the EventSource Last-Event-ID header.
// Empty / non-numeric values are treated as "no resume requested" and
// return 0 so the handler enters the live loop without replaying.
func parseLastEventIDHeader(v string) uint64 {
	if v == "" {
		return 0
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func (h *Handler) streamEvents(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.sse"
	r = calltrace.WithRequestRoot(r, op)
	debugHTTPRequest(r, op, "sse_accept", "text/event-stream")
	if h.hub == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "event stream unavailable")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("streaming unsupported", "cmd", calltrace.LogCmd, "operation", op, "err", errors.New("response writer is not an http.Flusher"))
		writeJSONError(w, r, op, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	setAPISecurityHeaders(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sinceID := parseLastEventIDHeader(r.Header.Get("Last-Event-ID"))
	sub, replay, hadGap, cancel := h.hub.subscribe(sinceID)
	defer cancel()

	if _, err := fmt.Fprintf(w, "retry: 3000\n\n"); err != nil {
		logSSEWriteError(r, op, err)
		return
	}
	flusher.Flush()

	if hadGap {
		if !writeResyncFrame(w, flusher, r, op) {
			return
		}
	} else if len(replay) > 0 {
		for _, ev := range replay {
			if !writeBufferedEvent(w, flusher, r, op, ev) {
				return
			}
		}
	}

	var heartbeat <-chan time.Time
	if h.hub.heartbeatPeriod > 0 {
		t := time.NewTicker(h.hub.heartbeatPeriod)
		defer t.Stop()
		heartbeat = t.C
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case <-sub.cancel:
			// Slow-consumer eviction. Tell the client to resync so
			// it reconnects with Last-Event-ID = our latest id; if
			// the gap is still beyond the ring it will fall through
			// to the gap-on-subscribe branch above and resync again.
			middleware.RecordSSEResyncEmitted(1)
			_ = writeResyncFrame(w, flusher, r, op)
			return
		case ev := <-sub.ch:
			if !writeBufferedEvent(w, flusher, r, op, ev) {
				return
			}
		case <-heartbeat:
			// Comment-line keep-alive — ignored by EventSource per
			// the SSE spec, but it keeps reverse proxies and load
			// balancers from idle-killing the TCP connection.
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				logSSEWriteError(r, op, err)
				return
			}
			flusher.Flush()
		}
	}
}

// writeBufferedEvent writes one event with its `id:` line so the
// browser EventSource captures it as Last-Event-ID for reconnect.
// Returns false (and logs) on write error so the caller can shut down.
func writeBufferedEvent(w http.ResponseWriter, flusher http.Flusher, r *http.Request, op string, ev bufferedEvent) bool {
	if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.id, ev.line); err != nil {
		logSSEWriteError(r, op, err)
		return false
	}
	flusher.Flush()
	return true
}

// writeResyncFrame emits a single `{"type":"resync"}` directive
// (without an id: line so EventSource does not advance its
// Last-Event-ID cursor — the client will treat the next reconnect as a
// "fresh" stream from the latest id).
func writeResyncFrame(w http.ResponseWriter, flusher http.Flusher, r *http.Request, op string) bool {
	middleware.RecordSSEResyncEmitted(1)
	if _, err := fmt.Fprintf(w, "data: {\"type\":\"resync\"}\n\n"); err != nil {
		logSSEWriteError(r, op, err)
		return false
	}
	flusher.Flush()
	return true
}

// logSSEWriteError records an unexpected SSE write failure. Client disconnects are silent
// (request context canceled) to avoid noise and duplicate logs with normal stream end.
func logSSEWriteError(r *http.Request, op string, err error) {
	if err == nil || r.Context().Err() != nil {
		return
	}
	slog.Log(r.Context(), slog.LevelWarn, "sse write failed", "cmd", calltrace.LogCmd, "operation", op, "err", err)
}

func (h *Handler) notifyChange(typ TaskChangeType, id string) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyChange", "change_type", typ)
	if h.hub == nil || id == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: typ, ID: id})
}

// notifyCycleChange publishes a `task_cycle_changed` event carrying both the
// owning task id and the affected cycle id. SPA cache invalidation hooks use
// the cycle id to scope their refetch instead of pulling the entire task tree.
// A blank taskID or cycleID is dropped (mirrors notifyChange's nil-hub guard).
func (h *Handler) notifyCycleChange(taskID, cycleID string) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyCycleChange", "task_id", taskID, "cycle_id", cycleID)
	if h.hub == nil || taskID == "" || cycleID == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: TaskCycleChanged, ID: taskID, CycleID: cycleID})
}

// legacySubscribeBuffer is the per-subscriber buffer the legacy in-process
// Subscribe() path uses. Pinned at 32 to preserve the original publish
// behavior the existing tests pin (silent drop after the bounded
// channel fills) — the legacy entry point predates Last-Event-ID
// resume and slow-consumer eviction, both of which only make sense
// over an HTTP /events connection. New consumers should subscribe
// through the HTTP handler instead.
const legacySubscribeBuffer = 32

// Subscribe is retained for in-process callers (tests and anything that
// wants the raw event stream without going through HTTP). It always
// starts at the live tail (no replay) and returns a string channel for
// backwards compatibility — every event is rendered as its JSON line.
//
// New SSE consumers should prefer subscribing through the HTTP handler
// so they pick up Last-Event-ID resume, heartbeats, and slow-consumer
// eviction. The string-channel surface here is kept stable for
// existing tests in pkgs/tasks/handler/sse_test.go and the wider
// SSE trigger suite.
func (h *SSEHub) Subscribe() (<-chan string, func()) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.SSEHub.Subscribe")
	sub := &subscriber{
		legacy: make(chan string, legacySubscribeBuffer),
		cancel: make(chan struct{}),
	}
	h.mu.Lock()
	h.subs[sub] = struct{}{}
	n := len(h.subs)
	h.mu.Unlock()
	middleware.RecordSSESubscriberGauge(n)
	cancel := func() {
		h.mu.Lock()
		_, ok := h.subs[sub]
		if ok {
			delete(h.subs, sub)
		}
		n := len(h.subs)
		h.mu.Unlock()
		middleware.RecordSSESubscriberGauge(n)
	}
	return sub.legacy, cancel
}
