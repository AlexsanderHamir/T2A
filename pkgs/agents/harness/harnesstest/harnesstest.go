// Package harnesstest provides shared harness integration test helpers
// used by harness_test and verify_test packages.
package harnesstest

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

const PollInterval = 10 * time.Millisecond

const defaultPollTimeout = 3 * time.Second

// DefaultPollTimeout is the default task-status poll window for harness tests.
const DefaultPollTimeout = defaultPollTimeout

// Env wires a fake store and cycle notifier for harness integration tests.
type Env struct {
	T           *testing.T
	Store       harness.Store
	Concrete    *store.Store
	Notifier    *notifierfake.RecordingCycleNotifier
	pollTimeout time.Duration
}

// EnvOption configures [NewEnv].
type EnvOption func(*Env)

// WithPollTimeout overrides the default status poll timeout.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func WithPollTimeout(d time.Duration) EnvOption {
	return func(e *Env) {
		e.pollTimeout = d
	}
}

// NewEnv returns a test environment with store and notifier fakes.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func NewEnv(t *testing.T, opts ...EnvOption) *Env {
	t.Helper()
	sf := storefake.New(t)
	e := &Env{
		T:           t,
		Store:       sf,
		Concrete:    sf.Store,
		Notifier:    notifierfake.NewRecordingCycleNotifier(),
		pollTimeout: defaultPollTimeout,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// CreateReadyTask seeds a task in ready status.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) CreateReadyTask(ctx context.Context, title string) *domain.Task {
	h.T.Helper()
	tsk, err := h.Store.Create(ctx, store.CreateTaskInput{
		Title:         title,
		InitialPrompt: "do the thing",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		h.T.Fatalf("create task: %v", err)
	}
	return tsk
}

// CreateReadyTaskWithModel seeds a ready task with a cursor model set.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) CreateReadyTaskWithModel(ctx context.Context, title, model string) *domain.Task {
	h.T.Helper()
	tsk, err := h.Store.Create(ctx, store.CreateTaskInput{
		Title:         title,
		InitialPrompt: "do the thing",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		CursorModel:   model,
	}, domain.ActorUser)
	if err != nil {
		h.T.Fatalf("create task: %v", err)
	}
	return tsk
}

// TransitionRunning moves the task to running status.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) TransitionRunning(ctx context.Context, tsk *domain.Task) *domain.Task {
	h.T.Helper()
	running := domain.StatusRunning
	updated, err := h.Store.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent)
	if err != nil {
		h.T.Fatalf("transition running: %v", err)
	}
	return updated
}

// NewHarness builds a harness with the env notifier when opts.Notifier is nil.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) NewHarness(r runner.Runner, opts harness.Options) *harness.Harness {
	h.T.Helper()
	if opts.Notifier == nil {
		opts.Notifier = h.Notifier
	}
	return harness.New(h.Store, r, opts)
}

// RunHarness invokes harness.Run asynchronously.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) RunHarness(ctx context.Context, hh *harness.Harness, tsk *domain.Task) <-chan struct{} {
	h.T.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		hh.Run(ctx, tsk)
	}()
	return done
}

// StartHarnessRun transitions the task to running then runs the harness.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) StartHarnessRun(ctx context.Context, tsk *domain.Task, r runner.Runner, opts harness.Options) <-chan struct{} {
	h.T.Helper()
	tsk = h.TransitionRunning(ctx, tsk)
	return h.RunHarness(ctx, h.NewHarness(r, opts), tsk)
}

// WaitTaskStatus polls until the task reaches want or times out.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (h *Env) WaitTaskStatus(ctx context.Context, taskID string, want domain.Status) *domain.Task {
	h.T.Helper()
	deadline := time.Now().Add(h.pollTimeout)
	for time.Now().Before(deadline) {
		got, err := h.Store.Get(ctx, taskID)
		if err == nil && got.Status == want {
			return got
		}
		time.Sleep(PollInterval)
	}
	got, _ := h.Store.Get(ctx, taskID)
	gotStatus := domain.Status("")
	if got != nil {
		gotStatus = got.Status
	}
	h.T.Fatalf("timeout waiting for task %s status=%q (last=%q)", taskID, want, gotStatus)
	return nil
}

// BlockingRunner blocks Run until Release is signaled or the context is cancelled.
type BlockingRunner struct {
	NameVal    string
	VersionVal string

	Starts chan runner.Request

	Release  chan struct{}
	Result   runner.Result
	Err      error
	PanicMsg string

	OnStart  func(req runner.Request)
	HonorCtx bool
}

// NewBlockingRunner returns a runner that blocks until Release is closed.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func NewBlockingRunner() *BlockingRunner {
	return &BlockingRunner{
		NameVal:    "block",
		VersionVal: "v0",
		Starts:     make(chan runner.Request, 4),
		Release:    make(chan struct{}),
		HonorCtx:   true,
	}
}

//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (b *BlockingRunner) Name() string { return b.NameVal }

//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (b *BlockingRunner) Version() string { return b.VersionVal }

//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (b *BlockingRunner) EffectiveModel(req runner.Request) string {
	return strings.TrimSpace(req.CursorModel)
}

//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (b *BlockingRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	b.Starts <- req
	if b.OnStart != nil {
		b.OnStart(req)
	}
	if b.PanicMsg != "" {
		panic(b.PanicMsg)
	}
	select {
	case <-b.Release:
		return b.Result, b.Err
	case <-ctx.Done():
		if b.HonorCtx {
			return runner.Result{}, fmt.Errorf("blocking runner cancelled: %w", runner.ErrTimeout)
		}
		return b.Result, b.Err
	}
}

// AssertCycleStatus checks cycle count and status for a task.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func AssertCycleStatus(t *testing.T, st harness.Store, taskID string, wantCount int, wantStatus domain.CycleStatus) *domain.TaskCycle {
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

// StatusSnappingNotifier records task status at each cycle publish.
type StatusSnappingNotifier struct {
	Store harness.Store

	mu       sync.Mutex
	statuses []domain.Status
	cycles   []string
}

// PublishCycleChange implements harness cycle notification.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (n *StatusSnappingNotifier) PublishCycleChange(taskID, cycleID string) {
	tsk, _ := n.Store.Get(context.Background(), taskID)
	var s domain.Status
	if tsk != nil {
		s = tsk.Status
	}
	n.mu.Lock()
	n.statuses = append(n.statuses, s)
	n.cycles = append(n.cycles, cycleID)
	n.mu.Unlock()
}

// Snapshot returns copies of recorded statuses and cycle IDs.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only harness helper; not part of production trace paths."
func (n *StatusSnappingNotifier) Snapshot() ([]domain.Status, []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := make([]domain.Status, len(n.statuses))
	cy := make([]string, len(n.cycles))
	copy(st, n.statuses)
	copy(cy, n.cycles)
	return st, cy
}
