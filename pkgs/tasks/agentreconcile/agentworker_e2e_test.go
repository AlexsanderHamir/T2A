package agentreconcile

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/worker"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

const (
	e2ePollInterval     = 10 * time.Millisecond
	e2ePollTimeout      = 3 * time.Second
	e2eReconcileTick    = 25 * time.Millisecond
	e2eIdleSettleWindow = 200 * time.Millisecond
)

// TestAgentWorkerE2E_readyTaskRunsThroughReconcileAndWorker is the
// V1 worker integration sweep (contract: docs/architecture.md): real
// SQLite store + bounded MemoryQueue + reconcile loop + worker +
// scripted fake runner. It enqueues a single ready task that the
// reconcile loop must surface (the test deliberately bypasses
// store.SetReadyTaskNotifier so the queue is empty until reconcile
// fills it), waits for the cycle to terminate, then asserts the full
// audit-row sequence and queue end state.
func TestAgentWorkerE2E_readyTaskRunsThroughReconcileAndWorker(t *testing.T) {
	t.Parallel()

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(4)

	_, _, wbID := seedAgentReconcileGit(t, st)
	tsk, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:            "e2e",
		InitialPrompt:    "do the thing",
		Status:           domain.StatusReady,
		Priority:         domain.PriorityMedium,
		WorktreeBranchID: &wbID,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create ready task: %v", err)
	}

	if got := q.BufferDepth(); got != 0 {
		t.Fatalf("queue depth before reconcile = %d, want 0 (notifier intentionally not wired)", got)
	}

	r := runnerfake.New().WithName("fake")
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "all green",
		json.RawMessage(`{"ok":true}`), "",
	))

	w := worker.NewWorker(st, q, r, worker.Options{
		RunTimeout: 30 * time.Second,
	})

	reconcileCtx, reconcileCancel := context.WithCancel(rootCtx)
	defer reconcileCancel()
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		agents.RunReconcileLoop(reconcileCtx, st, q, e2eReconcileTick, nil)
	}()

	workerCtx, workerCancel := context.WithCancel(rootCtx)
	defer workerCancel()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- w.Run(workerCtx)
	}()

	waitTaskStatusE2E(t, rootCtx, st, tsk.ID, domain.StatusDone)

	// Let the worker complete its post-TerminateCycle writes
	// (transitionTask + AckAfterRecv) before snapshotting queue state.
	time.Sleep(e2eIdleSettleWindow)

	if got := q.BufferDepth(); got != 0 {
		t.Fatalf("queue depth after run = %d, want 0", got)
	}

	cycles, err := st.ListCyclesForTask(rootCtx, tsk.ID, 10)
	if err != nil {
		t.Fatalf("list cycles: %v", err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1", len(cycles))
	}
	if cycles[0].Status != domain.CycleStatusSucceeded {
		t.Fatalf("cycle status = %q, want %q", cycles[0].Status, domain.CycleStatusSucceeded)
	}

	phases, err := st.ListPhasesForCycle(rootCtx, cycles[0].ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("phase count = %d, want 1 (execute)", len(phases))
	}
	if phases[0].Phase != domain.PhaseExecute || phases[0].Status != domain.PhaseStatusSucceeded {
		t.Fatalf("phase[0] = %q/%q, want execute/succeeded", phases[0].Phase, phases[0].Status)
	}

	events, err := st.ListTaskEvents(rootCtx, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	wantTypes := []domain.EventType{
		domain.EventCycleStarted,
		domain.EventPhaseStarted,
		domain.EventPhaseCompleted,
		domain.EventCycleCompleted,
	}
	gotSubset := filterEventTypes(events, wantTypes)
	if !sameOrderedTypes(gotSubset, wantTypes) {
		t.Fatalf("cycle/phase event sequence = %v, want %v (full=%v)",
			gotSubset, wantTypes, eventTypes(events))
	}

	if calls := r.Calls(); len(calls) != 1 {
		t.Fatalf("runner Run calls = %d, want 1", len(calls))
	}

	workerCancel()
	if err := <-workerDone; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	reconcileCancel()
	<-reconcileDone
}

// TestAgentWorkerE2E_worktreeBranchBinding verifies the worker resolves git
// context via worktree_branch_id and clears active_branch_id after the run.
func TestAgentWorkerE2E_worktreeBranchBinding(t *testing.T) {
	t.Parallel()

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(4)

	_, _, wbID := seedAgentReconcileGit(t, st)
	tsk, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:            "e2e-wb",
		InitialPrompt:    "via association",
		Status:           domain.StatusReady,
		Priority:         domain.PriorityMedium,
		WorktreeBranchID: &wbID,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create ready task: %v", err)
	}

	r := runnerfake.New().WithName("fake")
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "all green",
		json.RawMessage(`{"ok":true}`), "",
	))

	w := worker.NewWorker(st, q, r, worker.Options{
		RunTimeout: 30 * time.Second,
	})

	reconcileCtx, reconcileCancel := context.WithCancel(rootCtx)
	defer reconcileCancel()
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		agents.RunReconcileLoop(reconcileCtx, st, q, e2eReconcileTick, nil)
	}()

	workerCtx, workerCancel := context.WithCancel(rootCtx)
	defer workerCancel()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- w.Run(workerCtx)
	}()

	waitTaskStatusE2E(t, rootCtx, st, tsk.ID, domain.StatusDone)
	time.Sleep(e2eIdleSettleWindow)

	wb, err := st.GetWorktreeBranchByID(rootCtx, wbID)
	if err != nil {
		t.Fatalf("GetWorktreeBranchByID: %v", err)
	}
	wt, err := st.GetGitWorktreeByID(rootCtx, wb.WorktreeID)
	if err != nil {
		t.Fatalf("GetGitWorktreeByID: %v", err)
	}
	if wt.ActiveBranchID != nil {
		t.Fatalf("active_branch_id = %v after run, want cleared", *wt.ActiveBranchID)
	}

	workerCancel()
	if err := <-workerDone; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	reconcileCancel()
	<-reconcileDone
}

// TestAgentWorkerE2E_sameWorktreeDifferentBranchesSequential verifies sequential
// tasks on one worktree with different worktree_branch associations both run and
// clear active_branch_id between runs.
func TestAgentWorkerE2E_sameWorktreeDifferentBranchesSequential(t *testing.T) {
	t.Parallel()

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(4)

	wbMain, wbFeature := seedSameWorktreeTwoBranchAssocs(t, st)
	taskA, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:            "branch-main",
		InitialPrompt:    "on main",
		Status:           domain.StatusReady,
		Priority:         domain.PriorityMedium,
		WorktreeBranchID: &wbMain,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task A: %v", err)
	}
	taskB, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:            "branch-feature",
		InitialPrompt:    "on feature-b",
		Status:           domain.StatusReady,
		Priority:         domain.PriorityMedium,
		WorktreeBranchID: &wbFeature,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task B: %v", err)
	}

	r := runnerfake.New().WithName("fake")
	r.Script(taskA.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "main ok",
		json.RawMessage(`{"ok":true}`), "",
	))
	r.Script(taskB.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "feature ok",
		json.RawMessage(`{"ok":true}`), "",
	))

	w := worker.NewWorker(st, q, r, worker.Options{RunTimeout: 30 * time.Second})

	reconcileCtx, reconcileCancel := context.WithCancel(rootCtx)
	defer reconcileCancel()
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		agents.RunReconcileLoop(reconcileCtx, st, q, e2eReconcileTick, nil)
	}()

	workerCtx, workerCancel := context.WithCancel(rootCtx)
	defer workerCancel()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- w.Run(workerCtx)
	}()

	waitTaskStatusE2E(t, rootCtx, st, taskA.ID, domain.StatusDone)
	waitTaskStatusE2E(t, rootCtx, st, taskB.ID, domain.StatusDone)

	calls := r.Calls()
	if len(calls) != 2 {
		t.Fatalf("runner Run calls = %d, want 2", len(calls))
	}

	wb, err := st.GetWorktreeBranchByID(rootCtx, wbMain)
	if err != nil {
		t.Fatalf("GetWorktreeBranchByID: %v", err)
	}
	wt, err := st.GetGitWorktreeByID(rootCtx, wb.WorktreeID)
	if err != nil {
		t.Fatalf("GetGitWorktreeByID: %v", err)
	}
	if wt.ActiveBranchID != nil {
		t.Fatalf("active_branch_id = %v after runs, want cleared", *wt.ActiveBranchID)
	}

	workerCancel()
	if err := <-workerDone; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	reconcileCancel()
	<-reconcileDone
}

// TestAgentWorkerE2E_dependencyBlocksUntilUpstreamDone verifies the worker
// does not run a dependent task until every depends_on task is done.
func TestAgentWorkerE2E_dependencyBlocksUntilUpstreamDone(t *testing.T) {
	t.Parallel()

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)

	_, _, wbID := seedAgentReconcileGit(t, st)
	upstream, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:            "upstream",
		InitialPrompt:    "first",
		Status:           domain.StatusReady,
		Priority:         domain.PriorityMedium,
		WorktreeBranchID: &wbID,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	dependent, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:            "dependent",
		InitialPrompt:    "after upstream",
		Status:           domain.StatusReady,
		Priority:         domain.PriorityMedium,
		WorktreeBranchID: &wbID,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create dependent: %v", err)
	}
	if err := st.AddTaskDependency(rootCtx, dependent.ID, upstream.ID, domain.DependencySatisfiesDone); err != nil {
		t.Fatalf("add dependency: %v", err)
	}

	r := runnerfake.New().WithName("fake")
	r.Script(upstream.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "upstream ok",
		json.RawMessage(`{"ok":true}`), "",
	))
	r.Script(dependent.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "dependent ok",
		json.RawMessage(`{"ok":true}`), "",
	))

	w := worker.NewWorker(st, q, r, worker.Options{RunTimeout: 30 * time.Second})

	reconcileCtx, reconcileCancel := context.WithCancel(rootCtx)
	defer reconcileCancel()
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		agents.RunReconcileLoop(reconcileCtx, st, q, e2eReconcileTick, nil)
	}()

	workerCtx, workerCancel := context.WithCancel(rootCtx)
	defer workerCancel()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- w.Run(workerCtx)
	}()

	waitTaskStatusE2E(t, rootCtx, st, upstream.ID, domain.StatusDone)
	waitTaskStatusE2E(t, rootCtx, st, dependent.ID, domain.StatusDone)

	calls := r.Calls()
	if len(calls) != 2 {
		t.Fatalf("runner Run calls = %d, want 2 (upstream then dependent)", len(calls))
	}
	if calls[0].TaskID != upstream.ID {
		t.Fatalf("first runner call task_id = %q, want upstream %q", calls[0].TaskID, upstream.ID)
	}
	if calls[1].TaskID != dependent.ID {
		t.Fatalf("second runner call task_id = %q, want dependent %q", calls[1].TaskID, dependent.ID)
	}

	workerCancel()
	if err := <-workerDone; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	reconcileCancel()
	<-reconcileDone
}

func waitTaskStatusE2E(t *testing.T, ctx context.Context, st *store.Store, taskID string, want domain.Status) {
	t.Helper()
	deadline := time.Now().Add(e2ePollTimeout)
	for time.Now().Before(deadline) {
		got, err := st.Get(ctx, taskID)
		if err == nil && got.Status == want {
			return
		}
		time.Sleep(e2ePollInterval)
	}
	got, _ := st.Get(ctx, taskID)
	gotStatus := domain.Status("")
	if got != nil {
		gotStatus = got.Status
	}
	t.Fatalf("timeout waiting for task %s status=%q (last=%q)", taskID, want, gotStatus)
}

// filterEventTypes returns the subsequence of evs whose types appear in
// the want set, preserving event order.
func filterEventTypes(evs []domain.TaskEvent, want []domain.EventType) []domain.EventType {
	wantSet := make(map[domain.EventType]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	out := make([]domain.EventType, 0, len(evs))
	for _, e := range evs {
		if wantSet[e.Type] {
			out = append(out, e.Type)
		}
	}
	return out
}

func sameOrderedTypes(got, want []domain.EventType) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func eventTypes(evs []domain.TaskEvent) []domain.EventType {
	out := make([]domain.EventType, len(evs))
	for i, e := range evs {
		out[i] = e.Type
	}
	return out
}
