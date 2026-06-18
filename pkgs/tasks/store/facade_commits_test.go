package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestStore_UpsertAndListCycleCommits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "commits", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	entries := []store.CycleCommitEntry{
		{
			PhaseSeq:    1,
			Seq:         1,
			Repo:        "/repo",
			Worktree:    "/repo",
			Branch:      "main",
			SHA:         "abc1234567890abcdef1234567890abcdef1234",
			CommittedAt: when,
			Message:     "feat: first",
		},
	}
	if err := st.UpsertCycleCommits(ctx, tsk.ID, cycle.ID, entries); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, err := st.ListCommitsForCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1", len(rows))
	}
	if rows[0].SHA != entries[0].SHA || rows[0].Message != "feat: first" {
		t.Fatalf("row = %+v", rows[0])
	}

	entries = append(entries, store.CycleCommitEntry{
		PhaseSeq:    3,
		Seq:         2,
		Repo:        "/repo",
		Worktree:    "/repo",
		Branch:      "main",
		SHA:         "def1234567890abcdef1234567890abcdef12345",
		CommittedAt: when.Add(time.Minute),
		Message:     "fix: second",
	})
	if err := st.UpsertCycleCommits(ctx, tsk.ID, cycle.ID, entries); err != nil {
		t.Fatalf("upsert second batch: %v", err)
	}
	rows, err = st.ListCommitsForCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("list after upsert: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len after upsert = %d, want 2", len(rows))
	}
}
