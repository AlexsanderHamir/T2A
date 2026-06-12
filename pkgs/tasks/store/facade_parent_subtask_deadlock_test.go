package store_test

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks"
)

// TestEpicLifecycle_criteriaUnlockSubtasksThenParentAutoDone verifies the
// full epic scheduling model: subtasks dequeue on parent criteria_complete,
// parent stays not-done until all subtasks finish, then auto-completes.
func TestEpicLifecycle_criteriaUnlockSubtasksThenParentAutoDone(t *testing.T) {
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
	parentDep := domain.DependencyEdge{TaskID: parent.ID, Satisfies: domain.DependencySatisfiesCriteriaComplete}
	child, err := st.Create(ctx, store.CreateTaskInput{
		Title:         "child",
		InitialPrompt: "c",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		ParentID:      &parent.ID,
		DependsOn:     []domain.DependencyEdge{parentDep},
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	satisfied, err := tasks.DependenciesSatisfied(ctx, db, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if satisfied {
		t.Fatal("child should be blocked until parent criteria complete")
	}

	if err := st.SetChecklistItemDone(ctx, parent.ID, items[0].ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	satisfied, err = tasks.DependenciesSatisfied(ctx, db, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !satisfied {
		t.Fatal("child should be unblocked after parent criteria complete")
	}

	parentAfter, err := st.Get(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parentAfter.Status == domain.StatusDone {
		t.Fatal("parent must not be done while subtask is open")
	}
	if parentAfter.CriteriaSatisfiedAt == nil {
		t.Fatal("parent criteria_satisfied_at should be set")
	}

	done := domain.StatusDone
	if _, err := st.Update(ctx, parent.ID, store.UpdateTaskInput{Status: &done}, domain.ActorUser); err == nil {
		t.Fatal("parent should not reach done while subtask open")
	}

	if _, err := st.Update(ctx, child.ID, store.UpdateTaskInput{Status: &done}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}

	parentFinal, err := st.Get(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parentFinal.Status != domain.StatusDone {
		t.Fatalf("parent should auto-complete when last subtask done, got %s", parentFinal.Status)
	}
}
