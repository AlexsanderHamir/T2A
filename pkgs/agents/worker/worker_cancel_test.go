package worker_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestWorker_CancelCurrentRun_idleIsNoOp pins the documented contract
// that calling CancelCurrentRun when nothing is running returns false
// and does nothing observable. Lets the HTTP handler treat the call as
// idempotent without inspecting worker state first.
func TestWorker_CancelCurrentRun_idleIsNoOp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := runnerfake.New()
	w, done := h.startWorker(ctx, r, worker.Options{})

	if w.CancelCurrentRun() {
		t.Error("CancelCurrentRun() = true, want false (idle)")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}

// TestWorker_CancelCurrentRun_failsCycleWithOperatorReason hits the
// happy path of the new "Cancel current run" button:
//   - Worker picks up a ready task and the runner blocks.
//   - External caller invokes CancelCurrentRun (mimics POST
//     /settings/cancel-current-run from the SPA).
//   - Worker observes the cancelled ctx, marks the cycle
//     failed/cancelled_by_operator, transitions the task to failed,
//     and acks the queue so the next task can run.
//
// Pins the audit-trail invariant: when an operator cancels a run, the
// reason is "cancelled_by_operator" — NOT "runner_timeout" (which would
// imply the per-run cap fired) and NOT "shutdown" (which would imply
// the process is exiting). Distinguishing the three is the whole point
// of this branch in classifyRunOutcome.
func TestWorker_CancelCurrentRun_failsCycleWithOperatorReason(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "cancel-me")

	br := newBlockingRunner()
	w, done := h.startWorker(ctx, br, worker.Options{
		RunTimeout: 0,
	})

	select {
	case <-br.starts:
	case <-time.After(pollTimeout):
		t.Fatal("runner did not start; CancelCurrentRun would be a no-op")
	}

	if !w.CancelCurrentRun() {
		t.Fatal("CancelCurrentRun() = false, want true (a run is in flight)")
	}

	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusFailed)
	events, err := h.store.ListTaskEvents(context.Background(), tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var sawOperatorReason bool
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		if strings.Contains(string(e.Data), worker.CancelledByOperatorReason) {
			sawOperatorReason = true
			break
		}
	}
	if !sawOperatorReason {
		t.Fatalf("no cycle_failed event carried %q reason; events=%+v cycle=%s",
			worker.CancelledByOperatorReason, events, cycle.ID)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}

// TestWorker_NoCapRunTimeout_doesNotFireOnLongRun confirms RunTimeout
// <= 0 means "no cap": a runner that blocks for longer than the prior
// hard-coded 5-minute default still completes successfully when
// released. Without this guard the documented "No limit" UI option
// would silently degrade to a 5-minute cap because of the old
// fallback at NewWorker.
//
// We don't actually wait minutes — we simulate the no-cap contract by
// asserting the run completes when released and the cycle ends with
// reason="" (success), not "runner_timeout".
func TestWorker_NoCapRunTimeout_doesNotFireOnLongRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "no-cap")

	br := newBlockingRunner()
	br.result = runner.NewResult(domain.PhaseStatusSucceeded, "released", nil, "")

	_, done := h.startWorker(ctx, br, worker.Options{RunTimeout: 0})

	select {
	case <-br.starts:
	case <-time.After(pollTimeout):
		t.Fatal("runner did not start")
	}

	time.Sleep(150 * time.Millisecond)
	close(br.release)

	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done (no-cap run should succeed)", final.Status)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}

// TestWorker_PositiveRunTimeout_stillFiresAsTimeout pins the safety
// net for operators who explicitly set a Max run duration: a positive
// RunTimeout still fires and the cycle ends with reason="runner_timeout"
// (NOT "cancelled_by_operator" — no operator was involved).
func TestWorker_PositiveRunTimeout_stillFiresAsTimeout(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "timeout")

	br := newBlockingRunner()
	_, done := h.startWorker(ctx, br, worker.Options{RunTimeout: 50 * time.Millisecond})

	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	events, err := h.store.ListTaskEvents(context.Background(), tsk.ID)
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
		if strings.Contains(body, worker.CancelledByOperatorReason) {
			sawOperatorReason = true
		}
	}
	if !sawTimeoutReason {
		t.Errorf("expected cycle_failed event with reason=runner_timeout; events=%+v", events)
	}
	if sawOperatorReason {
		t.Errorf("cycle_failed carried %q reason without an operator cancel", worker.CancelledByOperatorReason)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}
