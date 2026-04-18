package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type recordedRun struct {
	Runner         string
	TerminalStatus string
	Duration       time.Duration
}

type recordingMetrics struct {
	mu    sync.Mutex
	calls []recordedRun
}

func (m *recordingMetrics) RecordRun(runnerName, terminalStatus string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, recordedRun{
		Runner:         runnerName,
		TerminalStatus: terminalStatus,
		Duration:       d,
	})
}

func (m *recordingMetrics) snapshot() []recordedRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]recordedRun, len(m.calls))
	copy(out, m.calls)
	return out
}

// TestWorker_RunMetrics_observesHappyPathOnce locks in that the
// happy-path TerminateCycle records exactly one observation with
// runner.Name() and the terminal cycle status.
func TestWorker_RunMetrics_observesHappyPathOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "metrics-happy")

	r := runnerfake.New().WithName("fake")
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ok",
		json.RawMessage(`{"ok":true}`), "",
	))

	metrics := &recordingMetrics{}
	_, done := h.startWorker(ctx, r, worker.Options{Metrics: metrics})

	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	calls := metrics.snapshot()
	if len(calls) != 1 {
		t.Fatalf("RecordRun calls = %d, want 1 (calls=%+v)", len(calls), calls)
	}
	if calls[0].Runner != "fake" {
		t.Fatalf("runner label = %q, want %q", calls[0].Runner, "fake")
	}
	if calls[0].TerminalStatus != string(domain.CycleStatusSucceeded) {
		t.Fatalf("terminal_status = %q, want %q",
			calls[0].TerminalStatus, domain.CycleStatusSucceeded)
	}
	if calls[0].Duration < 0 {
		t.Fatalf("duration = %s, must be >= 0", calls[0].Duration)
	}
}

// TestWorker_RunMetrics_observesRunnerFailure locks in that a typed
// runner.ErrNonZeroExit produces exactly one RecordRun with
// terminal_status="failed".
func TestWorker_RunMetrics_observesRunnerFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "metrics-fail")

	r := runnerfake.New().WithName("fake")
	r.FailWithResult(tsk.ID, domain.PhaseExecute,
		runner.NewResult(domain.PhaseStatusFailed, "exit 7",
			json.RawMessage(`{"exit_code":7}`), "stderr tail"),
		fmt.Errorf("cli exit: %w", runner.ErrNonZeroExit))

	metrics := &recordingMetrics{}
	_, done := h.startWorker(ctx, r, worker.Options{Metrics: metrics})

	h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	calls := metrics.snapshot()
	if len(calls) != 1 {
		t.Fatalf("RecordRun calls = %d, want 1 (calls=%+v)", len(calls), calls)
	}
	if calls[0].TerminalStatus != string(domain.CycleStatusFailed) {
		t.Fatalf("terminal_status = %q, want %q",
			calls[0].TerminalStatus, domain.CycleStatusFailed)
	}
}

// TestWorker_RunMetrics_observesShutdownAbort drives the parent ctx
// cancel mid-runner.Run path so handleShutdownAfterRun is the
// TerminateCycle site, and asserts it still records one observation
// with terminal_status="aborted".
func TestWorker_RunMetrics_observesShutdownAbort(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = h.createReadyTask(ctx, "metrics-shutdown")

	br := newBlockingRunner()
	br.onStart = func(req runner.Request) {
		// Cancel the worker ctx while invokeRunner is blocked; the
		// blocking runner honours ctx and returns wrapped
		// runner.ErrTimeout, but the worker checks parentCtx.Err()
		// first and routes to handleShutdownAfterRun.
		cancel()
	}
	br.result = runner.NewResult(domain.PhaseStatusSucceeded, "", nil, "")

	metrics := &recordingMetrics{}
	_, done := h.startWorker(ctx, br, worker.Options{Metrics: metrics})

	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("worker exit err: %v", err)
	}

	calls := metrics.snapshot()
	if len(calls) != 1 {
		t.Fatalf("RecordRun calls = %d, want 1 (calls=%+v)", len(calls), calls)
	}
	if calls[0].TerminalStatus != string(domain.CycleStatusAborted) {
		t.Fatalf("terminal_status = %q, want %q",
			calls[0].TerminalStatus, domain.CycleStatusAborted)
	}
}

// TestWorker_RunMetrics_nilMetricsIsNoop sanity-checks that the
// default Options{} (no Metrics) does not panic and the worker
// continues to terminate normally.
func TestWorker_RunMetrics_nilMetricsIsNoop(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "metrics-nil")

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ok", json.RawMessage(`{"ok":true}`), ""))

	_, done := h.startWorker(ctx, r, worker.Options{})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}
