package harness_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestHarness_CancelCurrentRun_idleIsNoOp(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	h := env.newHarness(runnerfake.New(), harness.Options{})

	if h.CancelCurrentRun() {
		t.Error("CancelCurrentRun() = true, want false (idle)")
	}
}

func TestHarness_CancelCurrentRun_failsCycleWithOperatorReason(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "cancel-me"))

	br := newBlockingRunner()
	h := env.newHarness(br, harness.Options{RunTimeout: 0})
	done := env.runHarness(ctx, h, tsk)

	select {
	case <-br.starts:
	case <-time.After(pollTimeout):
		t.Fatal("runner did not start; CancelCurrentRun would be a no-op")
	}

	if !h.CancelCurrentRun() {
		t.Fatal("CancelCurrentRun() = false, want true (a run is in flight)")
	}

	<-done
	final := env.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	cycle := assertCycleStatus(t, env.store, tsk.ID, 1, domain.CycleStatusFailed)
	events, err := env.store.ListTaskEvents(context.Background(), tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var sawOperatorReason bool
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		if strings.Contains(string(e.Data), harness.CancelledByOperatorReason) {
			sawOperatorReason = true
			break
		}
	}
	if !sawOperatorReason {
		t.Fatalf("no cycle_failed event carried %q reason; events=%+v cycle=%s",
			harness.CancelledByOperatorReason, events, cycle.ID)
	}
}

func TestHarness_NoCapRunTimeout_doesNotFireOnLongRun(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "no-cap"))

	br := newBlockingRunner()
	br.result = runner.NewResult(domain.PhaseStatusSucceeded, "released", nil, "")

	done := env.runHarness(ctx, env.newHarness(br, harness.Options{RunTimeout: 0}), tsk)

	select {
	case <-br.starts:
	case <-time.After(pollTimeout):
		t.Fatal("runner did not start")
	}

	time.Sleep(150 * time.Millisecond)
	close(br.release)

	<-done
	final := env.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done (no-cap run should succeed)", final.Status)
	}
}

func TestHarness_PositiveRunTimeout_stillFiresAsTimeout(t *testing.T) {
	t.Parallel()
	env := newHarnessWithFakes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := env.transitionRunning(ctx, env.createReadyTask(ctx, "timeout"))

	br := newBlockingRunner()
	done := env.runHarness(ctx, env.newHarness(br, harness.Options{RunTimeout: 50 * time.Millisecond}), tsk)

	<-done
	final := env.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	events, err := env.store.ListTaskEvents(context.Background(), tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var sawTimeoutReason, sawOperatorReason bool
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		body := string(e.Data)
		if strings.Contains(body, "runner_timeout") {
			sawTimeoutReason = true
		}
		if strings.Contains(body, harness.CancelledByOperatorReason) {
			sawOperatorReason = true
		}
	}
	if !sawTimeoutReason {
		t.Errorf("expected cycle_failed event with reason=runner_timeout; events=%+v", events)
	}
	if sawOperatorReason {
		t.Errorf("cycle_failed carried %q reason without an operator cancel", harness.CancelledByOperatorReason)
	}
}
