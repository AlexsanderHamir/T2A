package agentreconcile

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const (
	e2ePollInterval     = 10 * time.Millisecond
	e2ePollTimeout      = 3 * time.Second
	e2eReconcileTick    = 25 * time.Millisecond
	e2eIdleSettleWindow = 200 * time.Millisecond
)

// TestAgentWorkerE2E_readyTaskRunsThroughReconcileAndWorker is the
// V1 worker integration sweep (contract: docs/AGENT-WORKER.md): real
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

	tsk, err := st.Create(rootCtx, store.CreateTaskInput{
		Title:         "e2e",
		InitialPrompt: "do the thing",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
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
		agents.RunReconcileLoop(reconcileCtx, st, q, e2eReconcileTick)
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
	if len(phases) != 2 {
		t.Fatalf("phase count = %d, want 2 (diagnose+execute)", len(phases))
	}
	if phases[0].Phase != domain.PhaseDiagnose || phases[0].Status != domain.PhaseStatusSkipped {
		t.Fatalf("phase[0] = %q/%q, want diagnose/skipped", phases[0].Phase, phases[0].Status)
	}
	if phases[1].Phase != domain.PhaseExecute || phases[1].Status != domain.PhaseStatusSucceeded {
		t.Fatalf("phase[1] = %q/%q, want execute/succeeded", phases[1].Phase, phases[1].Status)
	}

	events, err := st.ListTaskEvents(rootCtx, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	wantTypes := []domain.EventType{
		domain.EventCycleStarted,
		domain.EventPhaseStarted,
		domain.EventPhaseSkipped,
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
