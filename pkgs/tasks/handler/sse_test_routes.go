package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const (
	sseTestEnvVar      = "T2A_SSE_TEST"
	sseTestIntervalVar = "T2A_SSE_TEST_INTERVAL"
	// fallbackTestTaskID is used when the DB has no tasks so the UI still receives a valid-shaped event.
	fallbackTestTaskID = "00000000-0000-0000-0000-000000000001"
)

// SSETestEnabled reports whether T2A_SSE_TEST=1 (dev-only SSE test routes and optional ticker).
func SSETestEnabled() bool {
	return strings.TrimSpace(os.Getenv(sseTestEnvVar)) == "1"
}

// RegisterSSETestRoutes registers GET /dev/sse/ping and POST /dev/sse/publish when enabled is true.
// Both endpoints inject synthetic TaskChangeEvent values into the hub (same wire format as real mutations).
func RegisterSSETestRoutes(mux *http.ServeMux, st *store.Store, hub *SSEHub, enabled bool) {
	if !enabled || mux == nil || st == nil || hub == nil {
		return
	}
	mux.HandleFunc("GET /dev/sse/ping", sseTestPing(st, hub))
	mux.HandleFunc("POST /dev/sse/publish", sseTestPublish(st, hub))
	slog.Info("sse test routes enabled", "cmd", httpLogCmd, "operation", "tasks.sse_test.register",
		"GET", "/dev/sse/ping", "POST", "/dev/sse/publish")
}

// RunSSETestTicker publishes task_updated on the hub at the given interval until the process exits.
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
			id := pickTestTaskID(ctx, st)
			hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: id})
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
			id = pickTestTaskID(ctx, st)
		}
		hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: id})
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
			id = pickTestTaskID(ctx, st)
		}
		hub.Publish(TaskChangeEvent{Type: typ, ID: id})
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

// pickTestTaskID returns the first task id in list order (id ASC, same as GET /tasks).
// Used by dev routes, the optional ticker, and POST /tasks when T2A_SSE_TEST=1.
func pickTestTaskID(ctx context.Context, st *store.Store) string {
	rows, err := st.List(ctx, 1, 0)
	if err != nil || len(rows) == 0 {
		return fallbackTestTaskID
	}
	return rows[0].ID
}
