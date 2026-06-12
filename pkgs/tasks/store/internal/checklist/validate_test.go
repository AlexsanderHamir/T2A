package checklist

import (
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestSetDoneWithEvidence_rejectsEmptyEvidence(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := t.Context()

	tsk := &domain.Task{
		ID: "task-1", Title: "t", InitialPrompt: "p", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}
	if err := db.WithContext(ctx).Create(tsk).Error; err != nil {
		t.Fatal(err)
	}
	it := &domain.TaskChecklistItem{ID: "item-1", TaskID: tsk.ID, SortOrder: 1, Text: "criterion"}
	if err := db.WithContext(ctx).Create(it).Error; err != nil {
		t.Fatal(err)
	}
	_, err := SetDoneWithEvidence(ctx, db, tsk.ID, it.ID, "", domain.VerifierAgentSelf, "", "", domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}
