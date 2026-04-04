package devsim

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// RunTicker starts a background goroutine: on each tick, calls PersistAllTasks with the given publish callback.
// Interval must be >= 1s; nil store or invalid interval is a no-op.
func RunTicker(st *store.Store, every time.Duration, publish func(taskID string)) {
	if st == nil || every < time.Second {
		return
	}
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		ctx := context.Background()
		for range tick.C {
			PersistAllTasks(ctx, st, publish)
		}
	}()
	slog.Info("sse dev ticker started", "cmd", logCmd, "operation", "devsim.ticker", "interval", every.String())
}
