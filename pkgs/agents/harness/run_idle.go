package harness

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
)

const (
	runStateProgressKind          = "run_state"
	runStateIdleSuspicious        = "idle_suspicious"
	runStateIdleKillPending       = "idle_kill_pending"
	runStateIdleRecovered         = "idle_recovered"
	RunnerStaleReason             = "runner_stale"
	defaultStreamIdleStuckSeconds = 60
)

func streamIdleProgressEvent(kind runner.StreamIdleKind, stuck time.Duration) runner.ProgressEvent {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.streamIdleProgressEvent",
		"kind", int(kind), "stuck_ns", int64(stuck))
	switch kind {
	case runner.StreamIdleKillPending:
		lead := 5 * time.Second
		if stuck > lead {
			return runner.ProgressEvent{
				Kind:    runStateProgressKind,
				Subtype: runStateIdleKillPending,
				Message: fmt.Sprintf("Terminating agent in %s if no output", lead.Round(time.Second)),
			}
		}
		return runner.ProgressEvent{
			Kind:    runStateProgressKind,
			Subtype: runStateIdleKillPending,
			Message: "Terminating agent soon if no output",
		}
	default:
		half := stuck / 2
		if half <= 0 {
			half = 30 * time.Second
		}
		return runner.ProgressEvent{
			Kind:    runStateProgressKind,
			Subtype: runStateIdleSuspicious,
			Message: fmt.Sprintf("No agent output for %s — run may be stuck", half.Round(time.Second)),
		}
	}
}

func streamIdleRecoveredEvent() runner.ProgressEvent {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.streamIdleRecoveredEvent")
	return runner.ProgressEvent{
		Kind:    runStateProgressKind,
		Subtype: runStateIdleRecovered,
		Message: "Agent went silent; recovered from saved evidence",
	}
}

func mergeStreamIdleRecoveryDetails(base []byte, stuck time.Duration) []byte {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.mergeStreamIdleRecoveryDetails",
		"base_bytes", len(base), "stuck_ns", int64(stuck))
	out := map[string]any{
		"stream_idle_recovery": true,
	}
	if stuck > 0 {
		out["stream_idle_stuck_seconds"] = int(stuck.Seconds())
	}
	if len(base) == 0 {
		b, _ := json.Marshal(out)
		return b
	}
	var existing map[string]any
	if err := json.Unmarshal(base, &existing); err != nil || existing == nil {
		b, _ := json.Marshal(out)
		return b
	}
	for k, v := range out {
		existing[k] = v
	}
	b, err := json.Marshal(existing)
	if err != nil {
		return base
	}
	return b
}

func (h *Harness) streamIdleRunnerFields(baseOnProgress func(runner.ProgressEvent)) (time.Duration, func(runner.StreamIdleKind)) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.streamIdleRunnerFields",
		"stuck_ns", int64(h.opts.StreamIdleStuck))
	stuck := h.opts.StreamIdleStuck
	if stuck <= 0 {
		return 0, nil
	}
	return stuck, func(kind runner.StreamIdleKind) {
		ev := streamIdleProgressEvent(kind, stuck)
		if baseOnProgress != nil {
			baseOnProgress(ev)
		}
	}
}
