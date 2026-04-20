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
	Model          string
	TerminalStatus string
	Duration       time.Duration
}

type recordingMetrics struct {
	mu    sync.Mutex
	calls []recordedRun
}

func (m *recordingMetrics) RecordRun(runnerName, model, terminalStatus string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, recordedRun{
		Runner:         runnerName,
		Model:          model,
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

// TestWorker_RunMetrics_recordsEffectiveModelLabel locks in Phase 3
// of the per-task runner+model attribution plan: the value
// runner.EffectiveModel(req) returned at startCycle MUST appear in
// the RecordRun observation, regardless of which terminate path
// fires. Exercises the happy path (req.CursorModel wins) and the
// runner-default path (empty req.CursorModel falls back to the
// runner's default), and also pins the empty-string case (no
// task model, no runner default) through as a verbatim empty
// label rather than a synthetic default.
func TestWorker_RunMetrics_recordsEffectiveModelLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		taskModel     string
		runnerDefault string
		wantModel     string
	}{
		{name: "task_wins_over_default", taskModel: "sonnet-4.5", runnerDefault: "opus-4", wantModel: "sonnet-4.5"},
		{name: "fallback_to_runner_default", taskModel: "", runnerDefault: "opus-4", wantModel: "opus-4"},
		{name: "no_model_configured_anywhere", taskModel: "", runnerDefault: "", wantModel: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tsk := h.createReadyTaskWithModel(ctx, "metrics-model-"+tc.name, tc.taskModel)

			r := runnerfake.New().WithName("fake").WithDefaultModel(tc.runnerDefault)
			r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
				domain.PhaseStatusSucceeded, "ok",
				json.RawMessage(`{"ok":true}`), ""))

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
			if calls[0].Model != tc.wantModel {
				t.Fatalf("model label = %q, want %q (taskModel=%q runnerDefault=%q)",
					calls[0].Model, tc.wantModel, tc.taskModel, tc.runnerDefault)
			}
			if calls[0].Runner != "fake" {
				t.Fatalf("runner label = %q, want %q", calls[0].Runner, "fake")
			}
		})
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
