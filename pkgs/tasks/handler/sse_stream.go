package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/middleware"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
			middleware.RecordSSEResyncEmitted(1)
			_ = writeResyncFrame(w, flusher, r, op)
			return
		case ev := <-sub.ch:
			if !writeBufferedEvent(w, flusher, r, op, ev) {
				return
			}
		case <-heartbeat:
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				logSSEWriteError(r, op, err)
				return
			}
			flusher.Flush()
		}
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func writeBufferedEvent(w http.ResponseWriter, flusher http.Flusher, r *http.Request, op string, ev bufferedEvent) bool {
	if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.id, ev.line); err != nil {
		logSSEWriteError(r, op, err)
		return false
	}
	flusher.Flush()
	return true
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func writeResyncFrame(w http.ResponseWriter, flusher http.Flusher, r *http.Request, op string) bool {
	middleware.RecordSSEResyncEmitted(1)
	if _, err := fmt.Fprintf(w, "data: {\"type\":\"resync\"}\n\n"); err != nil {
		logSSEWriteError(r, op, err)
		return false
	}
	flusher.Flush()
	return true
}

func logSSEWriteError(r *http.Request, op string, err error) {
	if err == nil || r.Context().Err() != nil {
		return
	}
	slog.Log(r.Context(), slog.LevelWarn, "sse write failed", "cmd", calltrace.LogCmd, "operation", op, "err", err)
}
