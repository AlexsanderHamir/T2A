package harness_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/worker"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

const pollInterval = 10 * time.Millisecond
const pollTimeout = 3 * time.Second

type testEnv struct {
	t        *testing.T
	store    *store.Store
	queue    *agents.MemoryQueue
	notifier *recordingNotifier
}

func newHarness(t *testing.T) *testEnv {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)
	st.SetReadyTaskNotifier(q)
	return &testEnv{t: t, store: st, queue: q, notifier: newRecordingNotifier()}
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

func (h *testEnv) startWorker(ctx context.Context, r runner.Runner, opts harness.Options) (*worker.Worker, <-chan error) {
	h.t.Helper()
	if opts.Notifier == nil {
		opts.Notifier = h.notifier
	}
	w := worker.NewWorker(h.store, h.queue, r, opts)
	done := make(chan error, 1)
	go func() {
		done <- w.Run(ctx)
	}()
	return w, done
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

type publishCall struct {
	TaskID  string
	CycleID string
}

type recordingNotifier struct {
	mu    sync.Mutex
	calls []publishCall
}

func newRecordingNotifier() *recordingNotifier {
	return &recordingNotifier{}
}

func (r *recordingNotifier) PublishCycleChange(taskID, cycleID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, publishCall{TaskID: taskID, CycleID: cycleID})
}

func (r *recordingNotifier) snapshot() []publishCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]publishCall, len(r.calls))
	copy(out, r.calls)
	return out
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

func assertCycleStatus(t *testing.T, st *store.Store, taskID string, wantCount int, wantStatus domain.CycleStatus) *domain.TaskCycle {
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
