package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestWorker_HappyPath_marksAllChecklistItemsDone pins the contract
// behind the user-visible bug where a task with done-criteria stayed
// stuck in `running` after a successful run because the worker never
// recorded checklist completions and ValidateCanMarkDoneInTx then
// rejected the StatusDone transition. The fix lives in
// process.go::completeChecklistOnSuccess; this test fails (task stays
// running, items stay open) if that helper is removed or stops being
// invoked on the success path.
func TestWorker_HappyPath_marksAllChecklistItemsDone(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "criteria-task")

	// Two user-defined criteria the agent must satisfy. Done flags
	// start false; the worker must flip both to true on a clean run.
	if _, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", domain.ActorUser); err != nil {
		t.Fatalf("add checklist item one: %v", err)
	}
	if _, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion two", domain.ActorUser); err != nil {
		t.Fatalf("add checklist item two: %v", err)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "all green",
		json.RawMessage(`{"ok":true}`), "",
	))

	_, done := h.startWorker(ctx, r, worker.Options{})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done", final.Status)
	}

	bg := context.Background()
	items, err := h.store.ListChecklistForSubject(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list checklist: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("checklist count = %d, want 2", len(items))
	}
	for _, it := range items {
		if !it.Done {
			t.Errorf("checklist item %q not marked done", it.Text)
		}
	}

	events, err := h.store.ListTaskEvents(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var toggled int
	for _, e := range events {
		if e.Type == domain.EventChecklistItemToggled {
			toggled++
		}
	}
	if toggled != 2 {
		t.Fatalf("checklist_item_toggled count = %d, want 2", toggled)
	}
}
