// Package metricsfake records harness RunMetrics calls for tests.
package metricsfake

import (
	"sync"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// RecordedRun is one RecordRun observation.
type RecordedRun struct {
	Runner         string
	Model          string
	TerminalStatus string
	Duration       time.Duration
}

// RecordedVerdict is one RecordVerifyVerdict observation.
type RecordedVerdict struct {
	Kind   domain.VerifierKind
	Passed bool
}

// RecordingMetrics implements harness.RunMetrics for tests.
type RecordingMetrics struct {
	mu             sync.Mutex
	calls          []RecordedRun
	verdicts       []RecordedVerdict
	verifyDuration []time.Duration
	verifyRetries  []int
}

// New constructs an empty recorder.
//
//funclogmeasure:skip category=tool-required-noop reason="Harness test fake only; run metrics are traced on production harness.Run chokepoints."
func New() *RecordingMetrics {
	return &RecordingMetrics{}
}

// RecordRun records a terminal cycle observation.
//
//funclogmeasure:skip category=hot-path reason="Test-only in-memory recorder; operation trace is emitted by harness.Run in production."
func (m *RecordingMetrics) RecordRun(runnerName, model, terminalStatus string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, RecordedRun{
		Runner:         runnerName,
		Model:          model,
		TerminalStatus: terminalStatus,
		Duration:       d,
	})
}

// RecordVerifyVerdict records a per-criterion verify outcome.
//
//funclogmeasure:skip category=hot-path reason="Test-only in-memory recorder; operation trace is emitted by harness.Run in production."
func (m *RecordingMetrics) RecordVerifyVerdict(kind domain.VerifierKind, passed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verdicts = append(m.verdicts, RecordedVerdict{Kind: kind, Passed: passed})
}

// ObserveVerifyDuration records verify phase wall clock.
//
//funclogmeasure:skip category=hot-path reason="Test-only in-memory recorder; operation trace is emitted by harness.Run in production."
func (m *RecordingMetrics) ObserveVerifyDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyDuration = append(m.verifyDuration, d)
}

// ObserveVerifyRetries records retry count on terminal cycles.
//
//funclogmeasure:skip category=hot-path reason="Test-only in-memory recorder; operation trace is emitted by harness.Run in production."
func (m *RecordingMetrics) ObserveVerifyRetries(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyRetries = append(m.verifyRetries, n)
}

// SnapshotRuns returns a copy of RecordRun calls.
//
//funclogmeasure:skip category=hot-path reason="Test-only assertion helper; no production I/O boundary."
func (m *RecordingMetrics) SnapshotRuns() []RecordedRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RecordedRun, len(m.calls))
	copy(out, m.calls)
	return out
}

// SnapshotVerdicts returns a copy of RecordVerifyVerdict calls.
//
//funclogmeasure:skip category=hot-path reason="Test-only assertion helper; no production I/O boundary."
func (m *RecordingMetrics) SnapshotVerdicts() []RecordedVerdict {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RecordedVerdict, len(m.verdicts))
	copy(out, m.verdicts)
	return out
}

// SnapshotVerifyDurations returns a copy of ObserveVerifyDuration calls.
//
//funclogmeasure:skip category=hot-path reason="Test-only assertion helper; no production I/O boundary."
func (m *RecordingMetrics) SnapshotVerifyDurations() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]time.Duration, len(m.verifyDuration))
	copy(out, m.verifyDuration)
	return out
}

// SnapshotVerifyRetries returns a copy of ObserveVerifyRetries calls.
//
//funclogmeasure:skip category=hot-path reason="Test-only assertion helper; no production I/O boundary."
func (m *RecordingMetrics) SnapshotVerifyRetries() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int, len(m.verifyRetries))
	copy(out, m.verifyRetries)
	return out
}
