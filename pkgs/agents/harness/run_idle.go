package harness

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
)

const (
	RunnerStaleReason = "runner_stale"
)

func streamIdleRecoveredEvent() runner.ProgressEvent {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.harness.streamIdleRecoveredEvent")
	return runner.ProgressEvent{
		Kind:    runner.ProgressRunStateKind,
		Subtype: runner.ProgressRunStateIdleRecovered,
		Message: "Agent went silent; recovered from saved evidence",
	}
}

func mergeStreamIdleRecoveryDetails(base []byte, stuck time.Duration) []byte {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.harness.mergeStreamIdleRecoveryDetails",
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.harness.Harness.streamIdleRunnerFields",
		"stuck_ns", int64(h.opts.StreamIdleStuck))
	stuck := h.opts.StreamIdleStuck
	if stuck <= 0 {
		return 0, nil
	}
	return stuck, func(kind runner.StreamIdleKind) {
		ev := runner.StreamIdleProgressEvent(kind, stuck)
		if baseOnProgress != nil {
			baseOnProgress(ev)
		}
	}
}
