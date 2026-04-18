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
// Interval must be >= 1s; nil store, nil publish, or invalid interval is a no-op.
// The goroutine exits when ctx is cancelled, so the caller controls its lifetime.
// A nil ctx is treated as context.Background() for backward compatibility, but in that case
// the goroutine will run until process exit; pass a cancelable ctx to avoid leaks.
func RunTicker(ctx context.Context, st *store.Store, every time.Duration, opts Options, publish func(ChangeKind, string)) {
	if st == nil || every < time.Second || publish == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var tickN atomic.Uint64
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("sse dev ticker stopped", "cmd", logCmd, "operation", "devsim.ticker",
					"reason", "ctx_done", "err", ctx.Err())
				return
			case <-tick.C:
				n := tickN.Add(1)
				if opts.LifecycleEnabled && opts.LifecycleEveryTicks > 0 &&
					n%uint64(opts.LifecycleEveryTicks) == 0 {
					RunLifecycleOnce(ctx, st, publish)
				}
				PersistAllTasks(ctx, st, opts, publish)
			}
		}
	}()
	slog.Info("sse dev ticker started", "cmd", logCmd, "operation", "devsim.ticker", "interval", every.String(),
		"sync_row", opts.SyncTaskRow, "events_per_tick", opts.EventsPerTick,
		"user_response", opts.UserResponse, "lifecycle", opts.LifecycleEnabled,
		"lifecycle_every", opts.LifecycleEveryTicks)
}
