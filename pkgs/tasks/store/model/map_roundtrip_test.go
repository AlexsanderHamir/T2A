package model

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

func TestAppSettings_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	cfg := datatypes.JSON(`{"cursor":{"binary_path":"/bin/cursor"}}`)
	orig := domain.AppSettings{
		ID:                          domain.AppSettingsRowID,
		AgentPaused:                 true,
		Runner:                      "cursor",
		CursorBin:                   "/bin/cursor",
		CursorModel:                 "opus",
		MaxRunDurationSeconds:       120,
		StreamIdleStuckSeconds:      45,
		AgentPickupDelaySeconds:     3,
		DisplayTimezone:             "America/Los_Angeles",
		OptimisticMutationsEnabled:  true,
		SSEReplayEnabled:            true,
		RunnerConfigs:               cfg,
		VerifyMaxRetries:            1,
		VerifyRunnerName:            "cursor",
		VerifyRunnerModel:           "gpt",
		VerifyCommandTimeoutSeconds: 90,
		CursorSessionResumeEnabled:  false,
		UpdatedAt:                   now,
	}
	m := FromDomainAppSettings(orig)
	back := ToDomainAppSettings(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch:\norig=%+v\nback=%+v", orig, back)
	}
	m2 := FromDomainAppSettings(back)
	if !appSettingsModelEqual(m, m2) {
		t.Fatalf("model round-trip mismatch")
	}
}

func appSettingsModelEqual(a, b AppSettings) bool {
	return reflect.DeepEqual(a, b)
}

func TestAppSettings_emptyRunnerConfigs(t *testing.T) {
	t.Parallel()
	orig := domain.DefaultAppSettings()
	m := FromDomainAppSettings(orig)
	m2 := FromDomainAppSettings(ToDomainAppSettings(m))
	if !reflect.DeepEqual(m, m2) {
		t.Fatal("empty runner configs round-trip failed")
	}
}

func TestTaskEvent_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	resp := "ack"
	respAt := now.Add(time.Minute)
	data := datatypes.JSON(`{"status":"ready"}`)
	thread := datatypes.JSON(`[{"at":"2026-03-01T12:01:00Z","by":"user","body":"hi"}]`)
	orig := domain.TaskEvent{
		TaskID:         "task-1",
		Seq:            2,
		At:             now,
		Type:           domain.EventStatusChanged,
		By:             domain.ActorUser,
		Data:           data,
		UserResponse:   &resp,
		UserResponseAt: &respAt,
		ResponseThread: thread,
	}
	m := FromDomainTaskEvent(orig)
	m2 := FromDomainTaskEvent(ToDomainTaskEvent(m))
	if !taskEventModelEqual(m, m2) {
		t.Fatal("model round-trip mismatch")
	}
	back := ToDomainTaskEvent(m)
	if !jsonEqual(data, back.Data) || !jsonEqual(thread, back.ResponseThread) {
		t.Fatalf("json columns diverged: data=%s thread=%s", back.Data, back.ResponseThread)
	}
	if back.UserResponse == nil || *back.UserResponse != resp {
		t.Fatalf("user response: got %v", back.UserResponse)
	}
}

func TestTaskEvent_nilOptionalFields(t *testing.T) {
	t.Parallel()
	orig := domain.TaskEvent{
		TaskID: "t",
		Seq:    1,
		At:     time.Now().UTC(),
		Type:   domain.EventTaskCreated,
		By:     domain.ActorAgent,
		Data:   datatypes.JSON(`{}`),
	}
	m := FromDomainTaskEvent(orig)
	m2 := FromDomainTaskEvent(ToDomainTaskEvent(m))
	if !taskEventModelEqual(m, m2) {
		t.Fatal("nil optional fields round-trip failed")
	}
}

func taskEventModelEqual(a, b TaskEvent) bool {
	return a.TaskID == b.TaskID &&
		a.Seq == b.Seq &&
		a.At.Equal(b.At) &&
		a.Type == b.Type &&
		a.By == b.By &&
		jsonEqual(a.Data, b.Data) &&
		jsonEqual(a.ResponseThread, b.ResponseThread) &&
		ptrStrEqual(a.UserResponse, b.UserResponse) &&
		ptrTimeEqual(a.UserResponseAt, b.UserResponseAt)
}

func jsonEqual(a, b datatypes.JSON) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var ja, jb any
	if err := json.Unmarshal(a, &ja); err != nil {
		return bytes.Equal(a, b)
	}
	if err := json.Unmarshal(b, &jb); err != nil {
		return bytes.Equal(a, b)
	}
	ma, _ := json.Marshal(ja)
	mb, _ := json.Marshal(jb)
	return bytes.Equal(ma, mb)
}

func ptrStrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func ptrTimeEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func TestTask_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	pid := "proj-1"
	wt := "wt-1"
	cm := "opus"
	gate := &domain.TaskGate{Status: domain.GateStatusPendingRelease, Hold: true}
	retry := &domain.PendingRetry{Mode: domain.RetryResume, ParentCycleID: "cyc-1"}
	orig := domain.Task{
		ID:              "task-1",
		Title:           "Ship it",
		Status:          domain.StatusReady,
		Priority:        domain.PriorityHigh,
		InitialPrompt:   "do the thing",
		ProjectID:       &pid,
		WorktreeID:      &wt,
		CursorModel:     cm,
		PickupNotBefore: &now,
		PendingRetry:    retry,
		Gate:            gate,
		Tags:            []string{"a", "b"},
		Milestone:       strPtr("m1"),
	}
	m := FromDomainTask(orig)
	back := ToDomainTask(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch:\norig=%+v\nback=%+v", orig, back)
	}
}

func TestTaskDependency_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	orig := domain.TaskDependency{
		TaskID:          "t1",
		DependsOnTaskID: "t0",
		Satisfies:       domain.DependencySatisfiesDone,
		CreatedAt:       now,
	}
	m := FromDomainTaskDependency(orig)
	back := ToDomainTaskDependency(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestProject_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	orig := domain.Project{
		ID:        "p1",
		Name:      "Alpha",
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m := FromDomainProject(orig)
	back := ToDomainProject(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func strPtr(s string) *string { return &s }

func TestTaskCycleCriteriaReport_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.TaskCycleCriteriaReport{
		ID:          "r1",
		CycleID:     "cyc-1",
		AttemptSeq:  domain.ExecuteCriteriaReportAttemptSeq,
		CriterionID: "crit-1",
		ClaimedDone: true,
		Evidence:    "done",
		WrittenAt:   now,
	}
	m := FromDomainTaskCycleCriteriaReport(orig)
	back := ToDomainTaskCycleCriteriaReport(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestTaskCycleVerifyReport_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.TaskCycleVerifyReport{
		ID:           "v1",
		CycleID:      "cyc-1",
		AttemptSeq:   1,
		CriterionID:  "crit-1",
		Verified:     true,
		VerifierKind: domain.VerifierVerifyAgent,
		Reasoning:    "looks good",
		WrittenAt:    now,
	}
	m := FromDomainTaskCycleVerifyReport(orig)
	back := ToDomainTaskCycleVerifyReport(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestTaskCycleCommandRun_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.TaskCycleCommandRun{
		ID:          "cmd-1",
		CycleID:     "cyc-1",
		AttemptSeq:  1,
		CriterionID: "crit-1",
		CommandSeq:  0,
		ExitCode:    0,
		MetaPath:    "/tmp/out.meta",
		WrittenAt:   now,
	}
	m := FromDomainTaskCycleCommandRun(orig)
	back := ToDomainTaskCycleCommandRun(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestTaskCycleCommit_roundTrip(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	recorded := when.Add(time.Minute)
	orig := domain.TaskCycleCommit{
		ID:          "c1",
		TaskID:      "task-1",
		CycleID:     "cyc-1",
		PhaseSeq:    1,
		Seq:         1,
		Repo:        "/repo",
		Worktree:    "/wt",
		Branch:      "main",
		SHA:         "abc123",
		CommittedAt: when,
		Message:     "fix",
		RecordedAt:  recorded,
	}
	m := FromDomainTaskCycleCommit(orig)
	back := ToDomainTaskCycleCommit(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestTaskDraft_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	payload := datatypes.JSON(`{"title":"draft"}`)
	orig := domain.TaskDraft{
		ID: "draft-1", Name: "My draft", PayloadJSON: payload,
		CreatedAt: now, UpdatedAt: now,
	}
	m := FromDomainTaskDraft(orig)
	back := ToDomainTaskDraft(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestTaskTemplate_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	payload := datatypes.JSON(`{"title":"template"}`)
	orig := domain.TaskTemplate{
		ID: "tmpl-1", Name: "My template", PayloadJSON: payload,
		CreatedAt: now, UpdatedAt: now,
	}
	m := FromDomainTaskTemplate(orig)
	back := ToDomainTaskTemplate(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestGitRepository_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.GitRepository{
		ID: "repo-1", Path: "/repo", GitCommonDir: "/repo/.git", HostPath: "/host/repo",
		DefaultBranch: "main", CreatedAt: now, UpdatedAt: now,
	}
	m := FromDomainGitRepository(orig)
	back := ToDomainGitRepository(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestGitWorktree_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.GitWorktree{
		ID: "wt-1", RepositoryID: "repo-1", Path: "/wt", Name: "main",
		IsMain: true, BranchID: "branch-1", CreatedAt: now,
	}
	m := FromDomainGitWorktree(orig)
	back := ToDomainGitWorktree(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestGitBranch_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.GitBranch{
		ID: "branch-1", RepositoryID: "repo-1", Name: "main",
		HeadSHA: "abc", CreatedAt: now,
	}
	m := FromDomainGitBranch(orig)
	back := ToDomainGitBranch(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}

func TestGitWorktree_branchID_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	orig := domain.GitWorktree{
		ID: "wt-1", RepositoryID: "repo-1", Path: "/repo/wt", Name: "wt",
		BranchID: "branch-1", IsMain: false, CreatedAt: now,
	}
	m := FromDomainGitWorktree(orig)
	back := ToDomainGitWorktree(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", orig, back)
	}
}
