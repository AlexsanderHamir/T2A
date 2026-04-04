package devsim

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// RunTicker starts a background goroutine: on each tick, optionally runs lifecycle simulation,
// then PersistAllTasks with the given publish callback.
// Interval must be >= 1s; nil store or invalid interval is a no-op.
func RunTicker(st *store.Store, every time.Duration, opts Options, publish func(ChangeKind, string)) {
	if st == nil || every < time.Second || publish == nil {
		return
	}
	var tickN atomic.Uint64
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		ctx := context.Background()
		for range tick.C {
			n := tickN.Add(1)
			if opts.LifecycleEnabled && opts.LifecycleEveryTicks > 0 &&
				n%uint64(opts.LifecycleEveryTicks) == 0 {
				RunLifecycleOnce(ctx, st, publish)
			}
			PersistAllTasks(ctx, st, opts, publish)
		}
	}()
	slog.Info("sse dev ticker started", "cmd", logCmd, "operation", "devsim.ticker", "interval", every.String(),
		"sync_row", opts.SyncTaskRow, "events_per_tick", opts.EventsPerTick,
		"user_response", opts.UserResponse, "lifecycle", opts.LifecycleEnabled,
		"lifecycle_every", opts.LifecycleEveryTicks)
}
