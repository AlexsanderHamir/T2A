package store

import (
	"context"
	"time"
)

// PickupWake schedules in-process wakeups for ready tasks whose
// pickup_not_before is in the future. Implementations live outside this
// package (typically pkgs/agents); the store holds an optional hook and
// calls Schedule/Cancel from CRUD paths. See docs/SCHEDULING.md and
// docs/AGENT-QUEUE.md.
type PickupWake interface {
	// Schedule registers (or replaces) a wake at notBefore UTC for taskID.
	Schedule(ctx context.Context, taskID string, notBefore time.Time)
	// Cancel removes any pending wake for taskID (no-op if none).
	Cancel(taskID string)
	// Stop releases timers and goroutines during process shutdown.
	Stop()
}
