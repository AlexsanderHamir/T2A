package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

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

// SSEHub fans out task change notifications to all connected SSE clients.
type SSEHub struct {
	mu   sync.RWMutex
	subs map[chan string]struct{}
}

// NewSSEHub returns a hub with no subscribers. It is safe for concurrent use.
func NewSSEHub() *SSEHub {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.NewSSEHub")
	return &SSEHub{subs: make(map[chan string]struct{})}
}

// Subscribe registers a subscriber. The returned channel receives JSON lines;
// cancel removes the subscriber and must be called when the HTTP request ends.
func (h *SSEHub) Subscribe() (<-chan string, func()) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.SSEHub.Subscribe")
	ch := make(chan string, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	n := len(h.subs)
	h.mu.Unlock()
	middleware.RecordSSESubscriberGauge(n)
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		n := len(h.subs)
		h.mu.Unlock()
		middleware.RecordSSESubscriberGauge(n)
	}
}

// Publish delivers one JSON-encoded event to all current subscribers (non-blocking per subscriber).
func (h *SSEHub) Publish(ev TaskChangeEvent) {
	if h == nil {
		return
	}
	b, err := json.Marshal(ev)
	if err != nil {
		slog.Error("sse publish marshal failed", "cmd", calltrace.LogCmd, "operation", "tasks.sse.publish", "err", err)
		return
	}
	line := string(b)
	h.mu.RLock()
	out := make([]chan string, 0, len(h.subs))
	for ch := range h.subs {
		out = append(out, ch)
	}
	h.mu.RUnlock()
	dropped := 0
	for _, ch := range out {
		select {
		case ch <- line:
		default:
			dropped++
		}
	}
	if dropped > 0 {
		middleware.RecordSSEDroppedFrames(dropped)
		slog.Warn("sse fanout dropped frames",
			"cmd", calltrace.LogCmd, "operation", "tasks.sse.publish",
			"event_type", ev.Type, "task_id", ev.ID,
			"subscribers", len(out), "dropped", dropped)
	}
	if len(out) > 0 {
		slog.Debug("sse fanout", "cmd", calltrace.LogCmd, "operation", "tasks.sse.publish",
			"event_type", ev.Type, "task_id", ev.ID, "subscribers", len(out), "dropped", dropped)
	}
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

	ch, cancel := h.hub.Subscribe()
	defer cancel()

	if _, err := fmt.Fprintf(w, "retry: 3000\n\n"); err != nil {
		logSSEWriteError(r, op, err)
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
				logSSEWriteError(r, op, err)
				return
			}
			flusher.Flush()
		}
	}
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
