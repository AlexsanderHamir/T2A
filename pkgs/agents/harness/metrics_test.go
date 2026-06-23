package harness_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/metricsfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestHarness_RunMetrics_observesHappyPathOnce(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "metrics-happy"))

	r := runnerfake.New().WithName("fake")
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ok",
		json.RawMessage(`{"ok":true}`), "",
	))

	metrics := metricsfake.New()
	done := env.runHarness(ctx, env.newHarness(r, harness.Options{Metrics: metrics}), tsk)
	<-done
	env.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)

	calls := metrics.SnapshotRuns()
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
}

func TestHarness_RunMetrics_observesRunnerFailure(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "metrics-fail"))

	r := runnerfake.New().WithName("fake")
	r.FailWithResult(tsk.ID, domain.PhaseExecute,
		runner.NewResult(domain.PhaseStatusFailed, "exit 7",
			json.RawMessage(`{"exit_code":7}`), "stderr tail"),
		fmt.Errorf("cli exit: %w", runner.ErrNonZeroExit))

	metrics := metricsfake.New()
	done := env.runHarness(ctx, env.newHarness(r, harness.Options{Metrics: metrics}), tsk)
	<-done
	env.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)

	calls := metrics.SnapshotRuns()
	if len(calls) != 1 {
		t.Fatalf("RecordRun calls = %d, want 1", len(calls))
	}
	if calls[0].TerminalStatus != string(domain.CycleStatusFailed) {
		t.Fatalf("terminal_status = %q, want %q",
			calls[0].TerminalStatus, domain.CycleStatusFailed)
	}
}

func TestHarness_RunMetrics_observesShutdownAbort(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "metrics-shutdown"))

	br := newBlockingRunner()
	br.onStart = func(req runner.Request) {
		cancel()
	}
	br.result = runner.NewResult(domain.PhaseStatusSucceeded, "", nil, "")

	metrics := metricsfake.New()
	done := env.runHarness(ctx, env.newHarness(br, harness.Options{Metrics: metrics}), tsk)
	<-done

	calls := metrics.SnapshotRuns()
	if len(calls) != 1 {
		t.Fatalf("RecordRun calls = %d, want 1 (calls=%+v)", len(calls), calls)
	}
	if calls[0].TerminalStatus != string(domain.CycleStatusAborted) {
		t.Fatalf("terminal_status = %q, want %q",
			calls[0].TerminalStatus, domain.CycleStatusAborted)
	}
}

func TestHarness_RunMetrics_recordsEffectiveModelLabel(t *testing.T) {
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
			env := newHarnessWithFakes(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tsk := env.transitionRunning(ctx, env.createReadyTaskWithModel(ctx, "metrics-model-"+tc.name, tc.taskModel))

			r := runnerfake.New().WithName("fake").WithDefaultModel(tc.runnerDefault)
			r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
				domain.PhaseStatusSucceeded, "ok",
				json.RawMessage(`{"ok":true}`), ""))

			metrics := metricsfake.New()
			done := env.runHarness(ctx, env.newHarness(r, harness.Options{Metrics: metrics}), tsk)
			<-done
			env.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)

			calls := metrics.SnapshotRuns()
			if len(calls) != 1 {
				t.Fatalf("RecordRun calls = %d, want 1", len(calls))
			}
			if calls[0].Model != tc.wantModel {
				t.Fatalf("model label = %q, want %q", calls[0].Model, tc.wantModel)
			}
		})
	}
}

func TestHarness_RunMetrics_nilMetricsIsNoop(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "metrics-nil"))

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ok", json.RawMessage(`{"ok":true}`), ""))

	done := env.runHarness(ctx, env.newHarness(r, harness.Options{}), tsk)
	<-done
	env.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
}
