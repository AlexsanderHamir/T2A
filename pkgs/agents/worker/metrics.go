package worker

import (
	"log/slog"
	"time"
)

// RunMetrics is the optional Prometheus seam for the agent worker.
// cmd/taskapi wires an adapter that increments a counter and observes
// a duration histogram; tests pass nil and every recordRun call
// becomes a no-op.
//
// Implementations MUST NOT block: the worker invokes RecordRun
// synchronously after each TerminateCycle write (happy path, panic,
// shutdown abort, and best-effort intermediate failures), so a slow
// metrics sink would back-pressure the run loop.
//
// terminalStatus is the string form of the terminal domain.CycleStatus
// (one of "succeeded", "failed", "aborted"). runner is whatever the
// adapter returned from runner.Runner.Name(). Cardinality is bounded
// because both label values come from a small fixed set; new runners
// add adapter implementations, not freeform labels.
type RunMetrics interface {
	RecordRun(runner string, terminalStatus string, duration time.Duration)
}

// recordRun fans out to the configured RunMetrics, if any. It is the
// single funnel for every terminal-cycle write in the worker so adding
// a new TerminateCycle call site cannot accidentally skip metrics.
//
// Defensive against zero/negative durations (e.g. tests with a fake
// clock that runs backwards): clamp to 0 so the histogram never sees a
// negative observation, and skip the record entirely when the worker
// did not actually start the cycle (state.startedAt zero) so we do not
// pollute the histogram with sub-millisecond garbage.
func (w *Worker) recordRun(terminalStatus, runnerName string, started time.Time) {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.Worker.recordRun",
		"terminal_status", terminalStatus, "runner", runnerName)
	if w == nil || w.options.Metrics == nil {
		return
	}
	if started.IsZero() {
		return
	}
	d := w.options.Clock().Sub(started)
	if d < 0 {
		d = 0
	}
	w.options.Metrics.RecordRun(runnerName, terminalStatus, d)
}
