package handler

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const sseTestEnvVar = "T2A_SSE_TEST"

// SSETestEnabled reports whether T2A_SSE_TEST=1 (starts the dev-only SSE ticker in taskapi).
func SSETestEnabled() bool {
	return strings.TrimSpace(os.Getenv(sseTestEnvVar)) == "1"
}

// RunSSETestTicker runs a background loop: on each tick, lists all tasks via store.List (same as GET /tasks
// pagination, max 200 per page) and for each row runs persistTaskUpdatedSSE (same path as PATCH /tasks) then SSE.
// No extra HTTP routes. Call only when SSETestEnabled(); interval must be >= 1s.
func RunSSETestTicker(st *store.Store, hub *SSEHub, every time.Duration) {
	if st == nil || hub == nil || every < time.Second {
		return
	}
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		ctx := context.Background()
		for range tick.C {
			persistAllTasksForSSETest(ctx, st, hub)
		}
	}()
	slog.Info("sse dev ticker started", "cmd", httpLogCmd, "operation", "tasks.sse_test.ticker", "interval", every.String())
}
