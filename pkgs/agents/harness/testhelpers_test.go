package harness_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/notifierfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/storefake"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

const pollInterval = 10 * time.Millisecond
const pollTimeout = 3 * time.Second

type testEnv struct {
	t        *testing.T
	store    harness.Store
	concrete *store.Store
	notifier *notifierfake.RecordingCycleNotifier
}

func newHarnessWithFakes(t *testing.T) *testEnv {
	t.Helper()
	sf := storefake.New(t)
	return &testEnv{
		t:        t,
		store:    sf,
		concrete: sf.Store,
		notifier: notifierfake.NewRecordingCycleNotifier(),
	}
}

func (h *testEnv) createReadyTask(ctx context.Context, title string) *domain.Task {
	h.t.Helper()
	tsk, err := h.store.Create(ctx, store.CreateTaskInput{
		Title:         title,
		InitialPrompt: "do the thing",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		h.t.Fatalf("create task: %v", err)
	}
	return tsk
}

func (h *testEnv) createReadyTaskWithModel(ctx context.Context, title, model string) *domain.Task {
	h.t.Helper()
	tsk, err := h.store.Create(ctx, store.CreateTaskInput{
		Title:         title,
		InitialPrompt: "do the thing",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		CursorModel:   model,
	}, domain.ActorUser)
	if err != nil {
		h.t.Fatalf("create task: %v", err)
	}
	return tsk
}

func (h *testEnv) transitionRunning(ctx context.Context, tsk *domain.Task) *domain.Task {
	h.t.Helper()
	running := domain.StatusRunning
	updated, err := h.store.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent)
	if err != nil {
		h.t.Fatalf("transition running: %v", err)
	}
	return updated
}

func (h *testEnv) newHarness(r runner.Runner, opts harness.Options) *harness.Harness {
	h.t.Helper()
	if opts.Notifier == nil {
		opts.Notifier = h.notifier
	}
	return harness.New(h.store, r, opts)
}

func (h *testEnv) runHarness(ctx context.Context, hh *harness.Harness, tsk *domain.Task) <-chan struct{} {
	h.t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		hh.Run(ctx, tsk)
	}()
	return done
}

func (h *testEnv) waitTaskStatus(ctx context.Context, taskID string, want domain.Status) *domain.Task {
	h.t.Helper()
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		got, err := h.store.Get(ctx, taskID)
		if err == nil && got.Status == want {
			return got
		}
		time.Sleep(pollInterval)
	}
	got, _ := h.store.Get(ctx, taskID)
	gotStatus := domain.Status("")
	if got != nil {
		gotStatus = got.Status
	}
	h.t.Fatalf("timeout waiting for task %s status=%q (last=%q)", taskID, want, gotStatus)
	return nil
}

type blockingRunner struct {
	name    string
	version string

	starts chan runner.Request

	release  chan struct{}
	result   runner.Result
	err      error
	panicMsg string

	onStart  func(req runner.Request)
	honorCtx bool
}

func newBlockingRunner() *blockingRunner {
	return &blockingRunner{
		name:     "block",
		version:  "v0",
		starts:   make(chan runner.Request, 4),
		release:  make(chan struct{}),
		honorCtx: true,
	}
}

func (b *blockingRunner) Name() string    { return b.name }
func (b *blockingRunner) Version() string { return b.version }

func (b *blockingRunner) EffectiveModel(req runner.Request) string {
	return strings.TrimSpace(req.CursorModel)
}

func (b *blockingRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	b.starts <- req
	if b.onStart != nil {
		b.onStart(req)
	}
	if b.panicMsg != "" {
		panic(b.panicMsg)
	}
	select {
	case <-b.release:
		return b.result, b.err
	case <-ctx.Done():
		if b.honorCtx {
			return runner.Result{}, fmt.Errorf("blocking runner cancelled: %w", runner.ErrTimeout)
		}
		return b.result, b.err
	}
}

func assertCycleStatus(t *testing.T, st harness.Store, taskID string, wantCount int, wantStatus domain.CycleStatus) *domain.TaskCycle {
	t.Helper()
	cycles, err := st.ListCyclesForTask(context.Background(), taskID, 10)
	if err != nil {
		t.Fatalf("list cycles: %v", err)
	}
	if len(cycles) != wantCount {
		t.Fatalf("cycle count = %d, want %d", len(cycles), wantCount)
	}
	if wantCount == 0 {
		return nil
	}
	c := cycles[0]
	if c.Status != wantStatus {
		t.Fatalf("cycle status = %q, want %q", c.Status, wantStatus)
	}
	return &c
}

// statusSnappingNotifier records task status at each cycle publish.
type statusSnappingNotifier struct {
	store harness.Store

	mu       sync.Mutex
	statuses []domain.Status
	cycles   []string
}

func (n *statusSnappingNotifier) PublishCycleChange(taskID, cycleID string) {
	tsk, _ := n.store.Get(context.Background(), taskID)
	var s domain.Status
	if tsk != nil {
		s = tsk.Status
	}
	n.mu.Lock()
	n.statuses = append(n.statuses, s)
	n.cycles = append(n.cycles, cycleID)
	n.mu.Unlock()
}

func (n *statusSnappingNotifier) snapshot() ([]domain.Status, []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := make([]domain.Status, len(n.statuses))
	cy := make([]string, len(n.cycles))
	copy(st, n.statuses)
	copy(cy, n.cycles)
	return st, cy
}
