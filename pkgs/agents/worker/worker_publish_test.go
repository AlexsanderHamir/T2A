package worker_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// statusSnappingNotifier records the *task row's status* at the moment
// each PublishCycleChange call lands. It exists to pin the bug fix in
// process.go where the worker now emits a trailing publish *after* the
// final transitionTask has flipped the row to its terminal status.
//
// Without that trailing publish the SPA's invalidation handler refetches
// the task on the cycle-terminate publish, races the in-flight task row
// transition, usually wins, and leaves the open detail page stuck on
// `running` until the user manually refreshes — which is exactly the
// user-visible regression this test guards.
type statusSnappingNotifier struct {
	store *store.Store

	mu       sync.Mutex
	statuses []domain.Status
	cycles   []string
}

func (n *statusSnappingNotifier) PublishCycleChange(taskID, cycleID string) {
	// Snapshot synchronously, *before* returning, so the recorded
	// status reflects what a SPA refetch would observe if it raced
	// the publish like the real frontend does.
	tsk, _ := n.store.Get(context.Background(), taskID)
	var s domain.Status
	if tsk != nil {
		s = tsk.Status
	}
	n.mu.Lock()
	n.statuses = append(n.statuses, s)
	n.cycles = append(n.cycles, cycleID)
	n.mu.Unlock()
}

func (n *statusSnappingNotifier) snapshot() ([]domain.Status, []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := make([]domain.Status, len(n.statuses))
	cy := make([]string, len(n.cycles))
	copy(st, n.statuses)
	copy(cy, n.cycles)
	return st, cy
}

// TestWorker_HappyPath_emitsTrailingPublishAfterTerminalStatus pins
// the contract that on a successful run, the *last* SSE publish lands
// after the task row has been transitioned to StatusDone. If the
// trailing `w.publish(task.ID, cycle.ID)` after transitionTask is
// removed from process.go, this test fails with the final snapshot
// recording StatusRunning instead of StatusDone — i.e. the exact
// stale-row-on-open-detail-page regression the user reported.
func TestWorker_HappyPath_emitsTrailingPublishAfterTerminalStatus(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "trailing-publish")

	snap := &statusSnappingNotifier{store: h.store}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "all green",
		json.RawMessage(`{"ok":true}`), "",
	))

	_, done := h.startWorker(ctx, r, worker.Options{Notifier: snap})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done", final.Status)
	}

	statuses, cycles := snap.snapshot()
	if len(statuses) == 0 {
		t.Fatal("notifier received zero publishes; expected at least one trailing publish after terminal status")
	}
	if got := statuses[len(statuses)-1]; got != domain.StatusDone {
		t.Fatalf("last publish observed task status = %q, want done; full snapshot=%+v", got, statuses)
	}
	// The trailing publish reuses the same cycle ID as the cycle's
	// terminate publish; the SPA only routes on task scope so this is
	// fine, but pinning it keeps a future refactor honest.
	if cycles[len(cycles)-1] == "" {
		t.Fatal("trailing publish used empty cycle id; SPA invalidation expects a populated frame")
	}
}
