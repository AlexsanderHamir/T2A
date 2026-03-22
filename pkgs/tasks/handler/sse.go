package handler

import (
	"encoding/json"
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
	return &SSEHub{subs: make(map[chan string]struct{})}
}

// Subscribe registers a subscriber. The returned channel receives JSON lines;
// cancel removes the subscriber and must be called when the HTTP request ends.
func (h *SSEHub) Subscribe() (<-chan string, func()) {
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
}

func (h *Handler) streamEvents(w http.ResponseWriter, r *http.Request) {
	const op = "tasks.sse"
	if h.hub == nil {
		http.Error(w, "event stream unavailable", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("streaming unsupported", "cmd", httpLogCmd, "operation", op)
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := h.hub.Subscribe()
	defer cancel()

	if _, err := fmt.Fprintf(w, "retry: 3000\n\n"); err != nil {
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
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Handler) notifyChange(typ TaskChangeType, id string) {
	if h.hub == nil || id == "" {
		return
	}
	h.hub.Publish(TaskChangeEvent{Type: typ, ID: id})
}
