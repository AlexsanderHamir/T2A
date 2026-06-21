package harness

import (
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// RunMetrics is the optional Prometheus seam for the agent harness.
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
// adapter returned from runner.Runner.Name(). model is whatever
// runner.EffectiveModel(req) returned for this cycle's request — may
// be empty when no model is configured anywhere; the adapter records
// "" verbatim rather than substituting a synthetic default. Per-runner
// label cardinality is bounded; per-model cardinality is shaped by
// the parallel `*_by_model_*` series only — see Phase 3 of the
// per-task runner+model attribution plan.
type RunMetrics interface {
	RecordRun(runner string, model string, terminalStatus string, duration time.Duration)
	// RecordVerifyVerdict is fired once per criterion verdict produced
	// by the verify pass. verifierKind is one of the
	// domain.VerifierKind values (deterministic_check, verify_agent,
	// agent_self) — see docs/data-model.md "verified_by" column.
	// passed is the verdict.
	//
	// This is the single counter for both verdict-rate dashboards and
	// disagreement queries: disagreement = the verifier rejecting a
	// criterion the execute agent claimed done, derivable as the
	// {verifier_kind="agent_self",verdict="failed"} slice. There is no
	// separate disagreement counter — one fact, one metric.
	RecordVerifyVerdict(verifierKind domain.VerifierKind, passed bool)
	// ObserveVerifyDuration receives one observation per cycle that
	// actually ran a verify phase (StartPhase(verify) → CompletePhase),
	// regardless of pass/fail/tampered outcome. Wall-clock duration
	// across deterministic checks, LLM verify, and integrity check.
	ObserveVerifyDuration(d time.Duration)
	// ObserveVerifyRetries receives one observation per terminal
	// cycle, value = number of verify retries the cycle consumed
	// (0 when the first verify attempt succeeded or verification was
	// skipped). The run histogram is the cycle distribution; alarm
	// signals look like p99 climbing over time as the verifier (or the
	// agent) drifts.
	ObserveVerifyRetries(n int)
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
func (h *Harness) recordRun(terminalStatus, runnerName, model string, started time.Time) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.recordRun",
		"terminal_status", terminalStatus, "runner", runnerName, "model", model)
	if h == nil || h.opts.Metrics == nil {
		return
	}
	if started.IsZero() {
		return
	}
	d := h.opts.Clock().Sub(started)
	if d < 0 {
		d = 0
	}
	h.opts.Metrics.RecordRun(runnerName, model, terminalStatus, d)
}

// recordVerifyVerdict fans the per-criterion verdict out to the
// configured RunMetrics. No-op when Metrics is nil.
func (h *Harness) recordVerifyVerdict(kind domain.VerifierKind, passed bool) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.recordVerifyVerdict",
		"verifier_kind", string(kind), "passed", passed)
	if h == nil || h.opts.Metrics == nil {
		return
	}
	h.opts.Metrics.RecordVerifyVerdict(kind, passed)
}

// observeVerifyDuration fans the verify-phase wall-clock observation
// out to the configured RunMetrics. d is clamped to >= 0 so a backwards
// fake clock cannot land a negative observation in the histogram.
func (h *Harness) observeVerifyDuration(d time.Duration) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.observeVerifyDuration",
		"duration_ms", d.Milliseconds())
	if h == nil || h.opts.Metrics == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	h.opts.Metrics.ObserveVerifyDuration(d)
}

// observeVerifyRetries records the per-cycle retry count. Called once
// from terminateCycle — same single funnel pattern as recordRun so
// adding a new TerminateCycle call site cannot accidentally skip
// metrics.
func (h *Harness) observeVerifyRetries(n int) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.Harness.observeVerifyRetries",
		"retries", n)
	if h == nil || h.opts.Metrics == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	h.opts.Metrics.ObserveVerifyRetries(n)
}
