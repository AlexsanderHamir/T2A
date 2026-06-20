package handlertest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskCommitsResponse struct {
	TaskID  string `json:"task_id"`
	Commits []struct {
		CycleID    string `json:"cycle_id"`
		AttemptSeq int64  `json:"attempt_seq"`
		SHA        string `json:"sha"`
		Message    string `json:"message"`
	} `json:"commits"`
}

func TestHandler_GetTaskCommits_returnsRows(t *testing.T) {
	t.Parallel()
	srv, st := NewServerWithStore(t)
	defer srv.Close()

	ctx := context.Background()
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Priority: domain.PriorityMedium, Title: "commits-api",
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	if err := st.UpsertCycleCommits(ctx, tsk.ID, cycle.ID, []store.CycleCommitEntry{
		{
			PhaseSeq: 1, Seq: 1, Repo: "/repo", Worktree: "/repo", Branch: "main",
			SHA: "abc1234567890abcdef1234567890abcdef1234", CommittedAt: when, Message: "feat",
		},
		{
			PhaseSeq: 1, Seq: 2, Repo: "/repo", Worktree: "/repo", Branch: "main",
			SHA: "def1234567890abcdef1234567890abcdef12345", CommittedAt: when.Add(time.Minute), Message: "fix",
		},
	}); err != nil {
		t.Fatalf("upsert commits: %v", err)
	}

	resp := getTaskCommits(t, srv.URL, tsk.ID)
	if resp.TaskID != tsk.ID {
		t.Fatalf("task_id=%q", resp.TaskID)
	}
	if len(resp.Commits) != 2 {
		t.Fatalf("len(commits)=%d want 2", len(resp.Commits))
	}
	if resp.Commits[0].SHA != "abc1234567890abcdef1234567890abcdef1234" || resp.Commits[0].AttemptSeq != cycle.AttemptSeq {
		t.Fatalf("first commit=%+v", resp.Commits[0])
	}
}

func TestHandler_GetTaskCommits_emptyForNewTask(t *testing.T) {
	t.Parallel()
	srv, st := NewServerWithStore(t)
	defer srv.Close()

	ctx := context.Background()
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Priority: domain.PriorityMedium, Title: "no-commits",
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	resp := getTaskCommits(t, srv.URL, tsk.ID)
	if len(resp.Commits) != 0 {
		t.Fatalf("commits=%+v want empty", resp.Commits)
	}
}

func getTaskCommits(t *testing.T, base, taskID string) taskCommitsResponse {
	t.Helper()
	url := fmt.Sprintf("%s/tasks/%s/commits", base, taskID)
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", res.StatusCode)
	}
	var out taskCommitsResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}
