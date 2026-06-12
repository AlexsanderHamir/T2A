package store_test

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks"
)

// TestUpdate_parentDoneUnblocksDependsOnSubtask verifies parent completion
// is criteria-driven, not blocked by open subtasks, and satisfies depends_on.
func TestUpdate_parentDoneUnblocksDependsOnSubtask(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()

	parent, err := st.Create(ctx, store.CreateTaskInput{
		Title: "parent", InitialPrompt: "p", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddChecklistItem(ctx, parent.ID, "parent deliverable exists", domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListChecklistForSubject(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("checklist items = %d want 1", len(items))
	}
	if err := st.SetChecklistItemDone(ctx, parent.ID, items[0].ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	child, err := st.Create(ctx, store.CreateTaskInput{
		Title:         "child",
		InitialPrompt: "c",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		ParentID:      &parent.ID,
		DependsOn:     []string{parent.ID},
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	satisfied, err := tasks.DependenciesSatisfied(ctx, db, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if satisfied {
		t.Fatal("child should be blocked until parent is done")
	}

	done := domain.StatusDone
	if _, err := st.Update(ctx, parent.ID, store.UpdateTaskInput{Status: &done}, domain.ActorUser); err != nil {
		t.Fatalf("parent with open subtask should reach done: %v", err)
	}

	satisfied, err = tasks.DependenciesSatisfied(ctx, db, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !satisfied {
		t.Fatal("child depends_on should be satisfied after parent is done")
	}
}
