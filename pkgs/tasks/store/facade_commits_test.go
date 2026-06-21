package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
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

func TestStore_ListCommitsForTask_dedupesAcrossCycles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "task commits", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	cycle1, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle1: %v", err)
	}
	sharedSHA := "abc1234567890abcdef1234567890abcdef1234"
	if err := st.UpsertCycleCommits(ctx, tsk.ID, cycle1.ID, []store.CycleCommitEntry{{
		PhaseSeq: 1, Seq: 1, Repo: "/repo", Worktree: "/repo", Branch: "main",
		SHA: sharedSHA, CommittedAt: when, Message: "attempt one",
	}}); err != nil {
		t.Fatalf("upsert cycle1: %v", err)
	}
	if _, err := st.TerminateCycle(ctx, cycle1.ID, domain.CycleStatusFailed, "test", domain.ActorAgent); err != nil {
		t.Fatalf("terminate cycle1: %v", err)
	}
	cycle2, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle2: %v", err)
	}
	if err := st.UpsertCycleCommits(ctx, tsk.ID, cycle2.ID, []store.CycleCommitEntry{
		{
			PhaseSeq: 1, Seq: 1, Repo: "/repo", Worktree: "/repo", Branch: "main",
			SHA: sharedSHA, CommittedAt: when, Message: "inherited duplicate",
		},
		{
			PhaseSeq: 1, Seq: 2, Repo: "/repo", Worktree: "/repo", Branch: "main",
			SHA: "def1234567890abcdef1234567890abcdef12345", CommittedAt: when.Add(time.Minute), Message: "attempt two",
		},
	}); err != nil {
		t.Fatalf("upsert cycle2: %v", err)
	}

	rows, err := st.ListCommitsForTask(ctx, tsk.ID)
	if err != nil {
		t.Fatalf("list task commits: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2 distinct shas", len(rows))
	}
	if rows[0].SHA != sharedSHA || rows[0].Message != "attempt one" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].Message != "attempt two" {
		t.Fatalf("second row = %+v", rows[1])
	}
}
