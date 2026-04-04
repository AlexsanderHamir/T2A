package devsim

import (
	"os"
	"strconv"
	"strings"
)

const (
	maxEventsPerTick = 50
)

// Options configures simulation behavior when T2A_SSE_TEST=1.
type Options struct {
	// SyncTaskRow mirrors task columns after each synthetic audit append (T2A_SSE_TEST_SYNC_ROW=1).
	SyncTaskRow bool
	// EventsPerTick is how many AppendTaskEvent calls per task per ticker fire (T2A_SSE_TEST_EVENTS_PER_TICK, 1–50).
	EventsPerTick int
	// UserResponse appends a synthetic thread message on approval_requested / task_failed rows (T2A_SSE_TEST_USER_RESPONSE=1).
	UserResponse bool
	// LifecycleEnabled creates/deletes tasks with id prefix t2a-devsim- (T2A_SSE_TEST_LIFECYCLE=1).
	LifecycleEnabled bool
	// LifecycleEveryTicks runs one create-or-delete attempt every N ticker fires (T2A_SSE_TEST_LIFECYCLE_EVERY, default 5).
	LifecycleEveryTicks int
}

// LoadOptions reads dev simulation tuning from the environment (safe to call when T2A_SSE_TEST is off).
func LoadOptions() Options {
	o := Options{
		EventsPerTick:       1,
		LifecycleEveryTicks: 5,
	}
	if envOne("T2A_SSE_TEST_SYNC_ROW") {
		o.SyncTaskRow = true
	}
	if envOne("T2A_SSE_TEST_USER_RESPONSE") {
		o.UserResponse = true
	}
	if envOne("T2A_SSE_TEST_LIFECYCLE") {
		o.LifecycleEnabled = true
	}
	if v := strings.TrimSpace(os.Getenv("T2A_SSE_TEST_EVENTS_PER_TICK")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			if n < 1 {
				n = 1
			}
			if n > maxEventsPerTick {
				n = maxEventsPerTick
			}
			o.EventsPerTick = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("T2A_SSE_TEST_LIFECYCLE_EVERY")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			o.LifecycleEveryTicks = n
		}
	}
	return o
}

func envOne(key string) bool {
	return strings.TrimSpace(os.Getenv(key)) == "1"
}
