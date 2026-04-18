package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestStore_EvaluateDraftTask_persists_multiple_evaluations(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()

	in := EvaluateDraftTaskInput{
		DraftID:       "draft-abc",
		Title:         "Harden task event pagination",
		InitialPrompt: "Add cursor tests for before_seq and after_seq",
		Priority:      domain.PriorityHigh,
	}
	a, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if a.EvaluationID == b.EvaluationID {
		t.Fatal("expected unique evaluation ids")
	}
	rows, err := s.ListDraftEvaluations(ctx, "draft-abc", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestStore_Create_attaches_draft_evaluations_to_task(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	in := EvaluateDraftTaskInput{
		DraftID:       "draft-link-1",
		Title:         "Link draft evals",
		InitialPrompt: "Persist and bind evaluations on create",
		Priority:      domain.PriorityMedium,
	}
	if _, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	task, err := s.Create(ctx, CreateTaskInput{
		Title:    "Final task",
		Priority: domain.PriorityMedium,
		DraftID:  "draft-link-1",
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListDraftEvaluations(ctx, "draft-link-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.TaskID == nil || *row.TaskID != task.ID {
			t.Fatalf("expected task_id %q, got %#v", task.ID, row.TaskID)
		}
	}
}
