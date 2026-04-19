package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestStore_SetChecklistItemDone_rejects_user_actor(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "criterion", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_SetChecklistItemDone_allows_agent(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "criterion", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].Done || items[0].ID != it.ID {
		t.Fatalf("checklist: %+v", items)
	}
}

// TestStore_SetChecklistItemDone_idempotent_skips_event pins the audit
// invariant: re-marking a checklist item done (or undone) when it is already
// in that state must NOT emit a new `checklist_item_toggled` event. This
// matches the established convention across every other patch path
// (`applyTitlePatch`, `applyInitialPromptPatch`, `applyChecklistInheritPatch`,
// `applyPriorityPatch`, `applyStatusPatch`, sibling `UpdateText`) which all
// short-circuit when the new value equals the current one — see comment on
// `UpdateText` ("idempotent UI saves do not pollute the audit log"). Before
// this fix `SetDone` always wrote a toggle event regardless of the
// before/after state, which (a) bloated the audit log on agent retries, (b)
// re-stamped the completion `at` timestamp on no-op done-true calls, and (c)
// triggered an SSE `task_updated` fanout for nothing.
func TestStore_SetChecklistItemDone_idempotent_skips_event(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "criterion", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	evs1, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var firstToggleSeq int64
	var firstToggleCount int
	for _, e := range evs1 {
		if e.Type == domain.EventChecklistItemToggled {
			firstToggleCount++
			firstToggleSeq = e.Seq
		}
	}
	if firstToggleCount != 1 {
		t.Fatalf("first SetDone(true) must emit exactly 1 toggle event, got %d; events=%+v", firstToggleCount, evs1)
	}
	// Re-mark done=true; should be a no-op (no second toggle event, no seq bump).
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	evs2, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var secondToggleCount int
	for _, e := range evs2 {
		if e.Type == domain.EventChecklistItemToggled {
			secondToggleCount++
		}
	}
	if secondToggleCount != 1 {
		t.Fatalf("second idempotent SetDone(true) must NOT emit a new toggle event; got %d toggle events; events=%+v", secondToggleCount, evs2)
	}
	if len(evs2) != len(evs1) {
		t.Fatalf("idempotent SetDone(true) must not append any event; before=%d after=%d", len(evs1), len(evs2))
	}
	// Now flip to done=false.
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, false, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	evs3, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var falseToggleCount int
	var lastFalseSeq int64
	for _, e := range evs3 {
		if e.Type == domain.EventChecklistItemToggled {
			falseToggleCount++
			lastFalseSeq = e.Seq
		}
	}
	if falseToggleCount != 2 {
		t.Fatalf("real flip true->false must emit a toggle event; want 2 total toggle events, got %d; events=%+v", falseToggleCount, evs3)
	}
	if lastFalseSeq <= firstToggleSeq {
		t.Fatalf("flip false event seq %d must be strictly greater than first toggle seq %d", lastFalseSeq, firstToggleSeq)
	}
	// Re-mark done=false; should also be a no-op.
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, false, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	evs4, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs4) != len(evs3) {
		t.Fatalf("idempotent SetDone(false) must not append any event; before=%d after=%d", len(evs3), len(evs4))
	}
}

func TestStore_UpdateChecklistItemText_updates_row(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "before", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChecklistItemText(ctx, tsk.ID, it.ID, "after", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Text != "after" {
		t.Fatalf("checklist: %+v", items)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range evs {
		if e.Type == domain.EventChecklistItemUpdated {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected checklist_item_updated event")
	}
}

// TestStore_UpdateChecklistItemText_rejects_done_item pins the rule
// that a checklist criterion can no longer be edited once the agent
// has marked it done. Without this guard a stale UI (or any other
// client) could rewrite the definition text after the fact, silently
// changing the meaning of the already-persisted
// EventChecklistItemToggled (done=true) row in the audit timeline.
// The SPA also disables its Edit button for done rows, but the
// authoritative rule lives here.
func TestStore_UpdateChecklistItemText_rejects_done_item(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "before", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorAgent); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	err = s.UpdateChecklistItemText(ctx, tsk.ID, it.ID, "after", domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
	// The persisted row must still hold the original definition text:
	// rejection has to be a no-op, not a partial write.
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Text != "before" {
		t.Fatalf("text changed despite rejection: %+v", items)
	}
	// And no spurious EventChecklistItemUpdated should have leaked
	// into the audit log — only the original add + the toggle.
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.Type == domain.EventChecklistItemUpdated {
			t.Fatalf("unexpected checklist_item_updated event after rejected edit: %+v", e)
		}
	}
}

func TestStore_UpdateChecklistItemText_rejects_checklist_inherit(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, parent.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.Create(ctx, CreateTaskInput{Title: "c", ParentID: &parent.ID, ChecklistInherit: true, Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpdateChecklistItemText(ctx, child.ID, it.ID, "nope", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_DeleteChecklistItem_appends_removed_event(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "gone", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteChecklistItem(ctx, tsk.ID, it.ID, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("checklist: %+v", items)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range evs {
		if e.Type == domain.EventChecklistItemRemoved {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected checklist_item_removed event")
	}
}
