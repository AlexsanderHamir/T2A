package store_test

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestValidateCanMarkDone_acceptsLegacyCompletions(t *testing.T) {
	t.Parallel()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()

	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "t", InitialPrompt: "p", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := st.AddChecklistItem(ctx, tsk.ID, "criterion", nil, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	done := domain.StatusDone
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &done}, domain.ActorAgent); err != nil {
		t.Fatalf("legacy completion should allow done: %v", err)
	}
}
