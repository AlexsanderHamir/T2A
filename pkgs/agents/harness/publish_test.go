package harness_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/harnesstest"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/notifierfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestHarness_HappyPath_emitsTrailingPublishAfterTerminalStatus(t *testing.T) {
	t.Parallel()
	env := harnesstest.NewEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.TransitionRunning(ctx, env.CreateReadyTask(ctx, "trailing-publish"))

	snap := &harnesstest.StatusSnappingNotifier{Store: env.Store}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "all green",
		json.RawMessage(`{"ok":true}`), "",
	))

	done := env.RunHarness(ctx, env.NewHarness(r, harness.Options{Notifier: snap}), tsk)
	<-done
	final := env.WaitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done", final.Status)
	}

	statuses, cycles := snap.Snapshot()
	if len(statuses) == 0 {
		t.Fatal("notifier received zero publishes")
	}
	if got := statuses[len(statuses)-1]; got != domain.StatusDone {
		t.Fatalf("last publish observed task status = %q, want done; full snapshot=%+v", got, statuses)
	}
	if cycles[len(cycles)-1] == "" {
		t.Fatal("trailing publish used empty cycle id")
	}
}

func TestHarness_PublishesRunnerProgressWithCycleAndPhaseContext(t *testing.T) {
	t.Parallel()
	env := harnesstest.NewEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.TransitionRunning(ctx, env.CreateReadyTask(ctx, "live-progress"))
	progress := notifierfake.NewRecordingProgressNotifier()
	r := harnesstest.NewBlockingRunner()
	r.Result = runner.NewResult(domain.PhaseStatusSucceeded, "all green", nil, "")
	r.OnStart = func(req runner.Request) {
		if req.OnProgress != nil {
			req.OnProgress(runner.ProgressEvent{
				Kind:    "tool_call",
				Subtype: "started",
				Tool:    "ReadFile",
				Message: "Started ReadFile",
				Payload: json.RawMessage(`{"type":"tool_call","name":"ReadFile","input":{"path":"README.md"}}`),
			})
		}
		close(r.Release)
	}

	done := env.RunHarness(ctx, env.NewHarness(r, harness.Options{ProgressNotifier: progress}), tsk)
	<-done
	env.WaitTaskStatus(ctx, tsk.ID, domain.StatusDone)

	calls := progress.Snapshot()
	if len(calls) != 1 {
		t.Fatalf("progress calls: got %d want 1 (%+v)", len(calls), calls)
	}
	got := calls[0]
	if got.TaskID != tsk.ID {
		t.Fatalf("TaskID: got %q want %q", got.TaskID, tsk.ID)
	}
	if got.CycleID == "" {
		t.Fatal("CycleID must be populated")
	}
	if got.PhaseSeq != 1 {
		t.Fatalf("PhaseSeq: got %d want 1", got.PhaseSeq)
	}
	if got.RunCorrelationID == "" {
		t.Fatal("RunCorrelationID must be populated")
	}
	stream, err := env.Store.ListCycleStreamEvents(context.Background(), got.CycleID, 0, 10)
	if err != nil {
		t.Fatalf("list persisted progress: %v", err)
	}
	if len(stream) != 1 {
		t.Fatalf("persisted stream events: got %d want 1", len(stream))
	}
}
