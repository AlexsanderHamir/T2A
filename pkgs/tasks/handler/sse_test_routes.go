package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const (
	sseTestEnvVar      = "T2A_SSE_TEST"
	sseTestIntervalVar = "T2A_SSE_TEST_INTERVAL"
)

// SSETestEnabled reports whether T2A_SSE_TEST=1 (dev-only SSE test routes and optional ticker).
func SSETestEnabled() bool {
	return strings.TrimSpace(os.Getenv(sseTestEnvVar)) == "1"
}

// RegisterSSETestRoutes registers GET /dev/sse/ping and POST /dev/sse/publish when enabled is true.
// task_updated paths persist via store.Update (same as PATCH) before broadcasting; task_created / task_deleted
// only send SSE frames (no DB writes) — use POST /tasks or DELETE /tasks for full persistence.
func RegisterSSETestRoutes(mux *http.ServeMux, st *store.Store, hub *SSEHub, enabled bool) {
	if !enabled || mux == nil || st == nil || hub == nil {
		return
	}
	mux.HandleFunc("GET /dev/sse/ping", sseTestPing(st, hub))
	mux.HandleFunc("POST /dev/sse/publish", sseTestPublish(st, hub))
	slog.Info("sse test routes enabled", "cmd", httpLogCmd, "operation", "tasks.sse_test.register",
		"GET", "/dev/sse/ping", "POST", "/dev/sse/publish")
}

// RunSSETestTicker persists a no-op task update and broadcasts task_updated on the given interval.
// Call only when SSETestEnabled() is true; interval should be >= 1s.
func RunSSETestTicker(st *store.Store, hub *SSEHub, every time.Duration) {
	if st == nil || hub == nil || every < time.Second {
		return
	}
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		ctx := context.Background()
		for range tick.C {
			id, ok := pickFirstTaskID(ctx, st)
			if !ok {
				continue
			}
			if err := persistTaskUpdatedSSE(ctx, st, hub, id); err != nil {
				slog.Debug("sse test tick skipped", "cmd", httpLogCmd, "operation", "tasks.sse_test.tick", "err", err)
			}
		}
	}()
	slog.Info("sse test ticker started", "cmd", httpLogCmd, "operation", "tasks.sse_test.ticker", "interval", every.String())
}

type sseTestPublishJSON struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func sseTestPing(st *store.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			var ok bool
			id, ok = pickFirstTaskID(ctx, st)
			if !ok {
				http.Error(w, "no tasks", http.StatusNotFound)
				return
			}
		}
		if err := persistTaskUpdatedSSE(ctx, st, hub, id); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				http.Error(w, "task not found", http.StatusNotFound)
				return
			}
			slog.Warn("sse test ping failed", "cmd", httpLogCmd, "operation", "tasks.sse_test.ping", "err", err)
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func sseTestPublish(st *store.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var body sseTestPublishJSON
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && err != io.EOF {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Type) == "" {
			body.Type = string(TaskUpdated)
		}
		typ, ok := parseTaskChangeType(strings.TrimSpace(body.Type))
		if !ok {
			http.Error(w, `type must be "task_created", "task_updated", or "task_deleted"`, http.StatusBadRequest)
			return
		}
		id := strings.TrimSpace(body.ID)
		if id == "" {
			var have bool
			id, have = pickFirstTaskID(ctx, st)
			if !have {
				http.Error(w, "no tasks", http.StatusNotFound)
				return
			}
		}
		switch typ {
		case TaskUpdated:
			if err := persistTaskUpdatedSSE(ctx, st, hub, id); err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					http.Error(w, "task not found", http.StatusNotFound)
					return
				}
				slog.Warn("sse test publish failed", "cmd", httpLogCmd, "operation", "tasks.sse_test.publish", "err", err)
				http.Error(w, "update failed", http.StatusInternalServerError)
				return
			}
		case TaskCreated, TaskDeleted:
			hub.Publish(TaskChangeEvent{Type: typ, ID: id})
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseTaskChangeType(s string) (TaskChangeType, bool) {
	switch s {
	case string(TaskCreated):
		return TaskCreated, true
	case string(TaskUpdated):
		return TaskUpdated, true
	case string(TaskDeleted):
		return TaskDeleted, true
	default:
		return "", false
	}
}
