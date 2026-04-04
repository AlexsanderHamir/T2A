package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// TaskChangeType names SSE payload types for task lifecycle.
type TaskChangeType string

const (
	TaskCreated TaskChangeType = "task_created"
	TaskUpdated TaskChangeType = "task_updated"
	TaskDeleted TaskChangeType = "task_deleted"
)

// TaskChangeEvent is a minimal JSON line sent as one SSE "data:" frame.
type TaskChangeEvent struct {
	Type TaskChangeType `json:"type"`
	ID   string         `json:"id"`
}

// SSEHub fans out task change notifications to all connected SSE clients.
type SSEHub struct {
	mu   sync.RWMutex
	subs map[chan string]struct{}
}

// NewSSEHub returns a hub with no subscribers. It is safe for concurrent use.
func NewSSEHub() *SSEHub {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.NewSSEHub")
	return &SSEHub{subs: make(map[chan string]struct{})}
}

// Subscribe registers a subscriber. The returned channel receives JSON lines;
// cancel removes the subscriber and must be called when the HTTP request ends.
func (h *SSEHub) Subscribe() (<-chan string, func()) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.SSEHub.Subscribe")
	ch := make(chan string, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// Publish delivers one JSON-encoded event to all current subscribers (non-blocking per subscriber).
func (h *SSEHub) Publish(ev TaskChangeEvent) {
	if h == nil {
		return
	}
	b, err := json.Marshal(ev)
	if err != nil {
		slog.Error("sse publish marshal failed", "cmd", httpLogCmd, "operation", "tasks.sse.publish", "err", err)
		return
	}
	line := string(b)
	h.mu.RLock()
	out := make([]chan string, 0, len(h.subs))
	for ch := range h.subs {
		out = append(out, ch)
	}
	h.mu.RUnlock()
	for _, ch := range out {
		select {
		case ch <- line:
		default:
		}
	}
	if len(out) > 0 {
		slog.Debug("sse fanout", "cmd", httpLogCmd, "operation", "tasks.sse.publish",
			"event_type", ev.Type, "task_id", ev.ID, "subscribers", len(out))
	}
}

func (h *Handler) streamEvents(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.sse"
	r = withCallRoot(r, op)
	debugHTTPRequest(r, op, "sse_accept", "text/event-stream")
	if h.hub == nil {
		writeJSONError(w, r, op, http.StatusServiceUnavailable, "event stream unavailable")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("streaming unsupported", "cmd", httpLogCmd, "operation", op, "err", errors.New("response writer is not an http.Flusher"))
		writeJSONError(w, r, op, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
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
	slog.Log(r.Context(), slog.LevelWarn, "sse write failed", "cmd", httpLogCmd, "operation", op, "err", err)
}

func (h *Handler) notifyChange(typ TaskChangeType, id string) {
	slog.Debug("trace", "cmd", httpLogCmd, "operation", "handler.Handler.notifyChange", "change_type", typ)
	if h.hub == nil || id == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: typ, ID: id})
}
