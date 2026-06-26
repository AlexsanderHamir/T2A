package runner

import (
	"fmt"
	"time"
)

const (
	ProgressRunStateKind            = "run_state"
	ProgressRunStateIdleSuspicious  = "idle_suspicious"
	ProgressRunStateIdleKillPending = "idle_kill_pending"
	ProgressRunStateIdleRecovered   = "idle_recovered"
)

// StreamIdleProgressEvent builds a live UI progress event for stdout-silence tiers.
func StreamIdleProgressEvent(kind StreamIdleKind, stuck time.Duration) ProgressEvent {
	switch kind {
	case StreamIdleKillPending:
		lead := 5 * time.Second
		if stuck > lead {
			return ProgressEvent{
				Kind:    ProgressRunStateKind,
				Subtype: ProgressRunStateIdleKillPending,
				Message: fmt.Sprintf("Terminating agent in %s if no output", lead.Round(time.Second)),
			}
		}
		return ProgressEvent{
			Kind:    ProgressRunStateKind,
			Subtype: ProgressRunStateIdleKillPending,
			Message: "Terminating agent soon if no output",
		}
	default:
		half := stuck / 2
		if half <= 0 {
			half = 30 * time.Second
		}
		return ProgressEvent{
			Kind:    ProgressRunStateKind,
			Subtype: ProgressRunStateIdleSuspicious,
			Message: fmt.Sprintf("No agent output for %s — run may be stuck", half.Round(time.Second)),
		}
	}
}
