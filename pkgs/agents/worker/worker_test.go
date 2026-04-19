package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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

const pollInterval = 10 * time.Millisecond
const pollTimeout = 3 * time.Second

// --- shared harness ------------------------------------------------------

type harness struct {
	t        *testing.T
	store    *store.Store
	queue    *agents.MemoryQueue
	notifier *recordingNotifier
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)
	st.SetReadyTaskNotifier(q)
	return &harness{t: t, store: st, queue: q, notifier: newRecordingNotifier()}
}

func (h *harness) createReadyTask(ctx context.Context, title string) *domain.Task {
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

func (h *harness) startWorker(ctx context.Context, r runner.Runner, opts worker.Options) (*worker.Worker, <-chan error) {
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

func (h *harness) waitTaskStatus(ctx context.Context, taskID string, want domain.Status) *domain.Task {
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

func (h *harness) waitNoCycle(ctx context.Context, taskID string) {
	h.t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		cycles, err := h.store.ListCyclesForTask(ctx, taskID, 10)
		if err == nil && len(cycles) == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if len(cycles) > 0 {
			h.t.Fatalf("expected no cycles for %s, got %d", taskID, len(cycles))
		}
		time.Sleep(pollInterval)
	}
}

// --- recording notifier -------------------------------------------------

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

// --- programmable runners ------------------------------------------------

type blockingRunner struct {
	name    string
	version string

	starts chan runner.Request

	// release is closed when the runner should return; result/err are
	// returned together. If panicMsg is non-empty the runner panics
	// after starts is signalled.
	release  chan struct{}
	result   runner.Result
	err      error
	panicMsg string

	// onStart is invoked synchronously after starts is signalled and
	// before the blocking select; tests use it to drive side effects
	// (delete the task, cancel parent ctx, etc).
	onStart func(req runner.Request)

	// honorCtx, when true, returns wrapped runner.ErrTimeout if ctx
	// fires while we are blocked (matches runnerfake semantics).
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

// --- helper assertions --------------------------------------------------

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

func eventTypeCounts(events []domain.TaskEvent) map[domain.EventType]int {
	out := map[domain.EventType]int{}
	for _, e := range events {
		out[e.Type]++
	}
	return out
}

// --- test cases ----------------------------------------------------------

func TestWorker_HappyPath_writesTwoPhasesAndSixMirrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "happy")

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "all green",
		json.RawMessage(`{"ok":true}`), "",
	))

	_, done := h.startWorker(ctx, r, worker.Options{})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	bg := context.Background()
	if final.Status != domain.StatusDone {
		t.Fatalf("task status = %q, want done", final.Status)
	}

	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusSucceeded)

	var meta map[string]string
	if err := json.Unmarshal(cycle.MetaJSON, &meta); err != nil {
		t.Fatalf("unmarshal meta: %v (raw=%s)", err, cycle.MetaJSON)
	}
	if meta["runner"] != "fake" || meta["runner_version"] != "v0" || len(meta["prompt_hash"]) != 64 {
		t.Fatalf("meta json shape = %+v", meta)
	}

	phases, err := h.store.ListPhasesForCycle(bg, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	if len(phases) != 2 {
		t.Fatalf("phase count = %d, want 2", len(phases))
	}
	if phases[0].Phase != domain.PhaseDiagnose || phases[0].Status != domain.PhaseStatusSkipped {
		t.Fatalf("phase[0] = %q/%q, want diagnose/skipped", phases[0].Phase, phases[0].Status)
	}
	if phases[0].Summary == nil || *phases[0].Summary != worker.SkippedDiagnoseSummary {
		t.Fatalf("phase[0].summary = %v, want %q", phases[0].Summary, worker.SkippedDiagnoseSummary)
	}
	if phases[1].Phase != domain.PhaseExecute || phases[1].Status != domain.PhaseStatusSucceeded {
		t.Fatalf("phase[1] = %q/%q, want execute/succeeded", phases[1].Phase, phases[1].Status)
	}

	events, err := h.store.ListTaskEvents(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	counts := eventTypeCounts(events)
	if counts[domain.EventCycleStarted] != 1 {
		t.Fatalf("cycle_started count = %d, want 1", counts[domain.EventCycleStarted])
	}
	if counts[domain.EventCycleCompleted] != 1 {
		t.Fatalf("cycle_completed count = %d, want 1", counts[domain.EventCycleCompleted])
	}
	if counts[domain.EventPhaseStarted] != 2 {
		t.Fatalf("phase_started count = %d, want 2", counts[domain.EventPhaseStarted])
	}
	if counts[domain.EventPhaseSkipped] != 1 {
		t.Fatalf("phase_skipped count = %d, want 1", counts[domain.EventPhaseSkipped])
	}
	if counts[domain.EventPhaseCompleted] != 1 {
		t.Fatalf("phase_completed count = %d, want 1", counts[domain.EventPhaseCompleted])
	}

	calls := h.notifier.snapshot()
	if len(calls) != 6 {
		t.Fatalf("notifier publish count = %d, want 6 (calls=%+v)", len(calls), calls)
	}
	for i, c := range calls {
		if c.TaskID != tsk.ID || c.CycleID != cycle.ID {
			t.Fatalf("publish[%d] = %+v, want task=%s cycle=%s", i, c, tsk.ID, cycle.ID)
		}
	}
}

func TestWorker_RunnerFailure_marksCycleAndTaskFailed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "boom")

	r := runnerfake.New()
	r.FailWithResult(tsk.ID, domain.PhaseExecute,
		runner.NewResult(domain.PhaseStatusFailed, "exit 7", json.RawMessage(`{"exit_code":7}`), "stderr tail"),
		fmt.Errorf("cli exit: %w", runner.ErrNonZeroExit))

	_, done := h.startWorker(ctx, r, worker.Options{})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	bg := context.Background()
	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusFailed)
	phases, _ := h.store.ListPhasesForCycle(bg, cycle.ID)
	if len(phases) != 2 {
		t.Fatalf("phase count = %d, want 2", len(phases))
	}
	if phases[1].Phase != domain.PhaseExecute || phases[1].Status != domain.PhaseStatusFailed {
		t.Fatalf("execute phase = %q/%q, want execute/failed", phases[1].Phase, phases[1].Status)
	}

	events, _ := h.store.ListTaskEvents(bg, tsk.ID)
	counts := eventTypeCounts(events)
	if counts[domain.EventCycleFailed] != 1 {
		t.Fatalf("cycle_failed count = %d, want 1", counts[domain.EventCycleFailed])
	}
	if counts[domain.EventPhaseFailed] != 1 {
		t.Fatalf("phase_failed count = %d, want 1", counts[domain.EventPhaseFailed])
	}

	if got := h.queue.BufferDepth(); got != 0 {
		t.Fatalf("queue depth = %d, want 0 (acked)", got)
	}
}

func TestWorker_StaleTaskAtDequeue_ackAndSkip(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "stale")

	// Move the task off `ready` AFTER it was enqueued by Create.
	doneStatus := domain.StatusDone
	if _, err := h.store.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &doneStatus}, domain.ActorUser); err != nil {
		t.Fatalf("update to done: %v", err)
	}

	r := runnerfake.New()
	_, done := h.startWorker(ctx, r, worker.Options{})

	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) && h.queue.BufferDepth() > 0 {
		time.Sleep(pollInterval)
	}
	if h.queue.BufferDepth() != 0 {
		t.Fatalf("queue still has %d after stale dequeue", h.queue.BufferDepth())
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	h.waitNoCycle(ctx, tsk.ID)
	if calls := h.notifier.snapshot(); len(calls) != 0 {
		t.Fatalf("notifier publish count = %d, want 0 on stale", len(calls))
	}
	if calls := r.Calls(); len(calls) != 0 {
		t.Fatalf("runner Run calls = %d, want 0 on stale", len(calls))
	}
}

func TestWorker_TaskDeletedMidCycle_logsAndAcks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "deleteme")

	br := newBlockingRunner()
	br.onStart = func(req runner.Request) {
		// Cascade-deletes the cycle + phase rows.
		if _, _, err := h.store.Delete(context.Background(), tsk.ID, domain.ActorUser); err != nil {
			t.Logf("delete during run: %v", err)
		}
		close(br.release)
	}
	br.result = runner.NewResult(domain.PhaseStatusSucceeded, "", nil, "")

	_, done := h.startWorker(ctx, br, worker.Options{})

	bg := context.Background()
	deadline := time.Now().Add(pollTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		_, err := h.store.Get(bg, tsk.ID)
		if errors.Is(err, domain.ErrNotFound) {
			lastErr = err
			break
		}
		lastErr = err
		time.Sleep(pollInterval)
	}
	if !errors.Is(lastErr, domain.ErrNotFound) {
		t.Fatalf("expected task deleted, last err=%v", lastErr)
	}

	deadline = time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) && h.queue.BufferDepth() > 0 {
		time.Sleep(pollInterval)
	}
	if h.queue.BufferDepth() != 0 {
		t.Fatalf("queue depth = %d, want 0 after delete", h.queue.BufferDepth())
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}

func TestWorker_PanicInRunner_terminatesAndContinues(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := h.createReadyTask(ctx, "panicker")
	second := h.createReadyTask(ctx, "after-panic")

	r := runnerfake.New()
	r.Script(second.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ok", nil, ""))

	// Wrap the fake to panic on the first task and delegate to the
	// fake on the second so the loop must keep going.
	pr := &panickyRunner{
		Runner:    r,
		panicTask: first.ID,
	}

	_, done := h.startWorker(ctx, pr, worker.Options{})

	h.waitTaskStatus(ctx, first.ID, domain.StatusFailed)
	h.waitTaskStatus(ctx, second.ID, domain.StatusDone)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	bg := context.Background()
	c := assertCycleStatus(t, h.store, first.ID, 1, domain.CycleStatusFailed)
	phases, _ := h.store.ListPhasesForCycle(bg, c.ID)
	if len(phases) != 2 {
		t.Fatalf("panic cycle phase count = %d, want 2", len(phases))
	}
	if phases[1].Phase != domain.PhaseExecute || phases[1].Status != domain.PhaseStatusFailed {
		t.Fatalf("execute phase after panic = %q/%q, want execute/failed", phases[1].Phase, phases[1].Status)
	}

	events, _ := h.store.ListTaskEvents(bg, first.ID)
	counts := eventTypeCounts(events)
	if counts[domain.EventCycleFailed] != 1 || counts[domain.EventPhaseFailed] != 1 {
		t.Fatalf("panic cycle event counts = %+v", counts)
	}
}

type panickyRunner struct {
	*runnerfake.Runner
	panicTask string
}

func (p *panickyRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	if req.TaskID == p.panicTask {
		panic("worker test induced panic")
	}
	return p.Runner.Run(ctx, req)
}

func TestWorker_ShutdownMidRun_writesAbortedCycleAndFailedTask(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "shutdown")

	br := newBlockingRunner()
	cancelOnce := sync.Once{}
	br.onStart = func(req runner.Request) {
		cancelOnce.Do(func() {
			cancel()
		})
	}

	_, done := h.startWorker(ctx, br, worker.Options{
		ShutdownAbortTimeout: 2 * time.Second,
	})

	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	bg := context.Background()
	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusAborted)

	final, err := h.store.Get(bg, tsk.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status after shutdown = %q, want failed", final.Status)
	}

	phases, _ := h.store.ListPhasesForCycle(bg, cycle.ID)
	if len(phases) != 2 {
		t.Fatalf("phase count after shutdown = %d, want 2", len(phases))
	}
	if phases[1].Phase != domain.PhaseExecute || phases[1].Status != domain.PhaseStatusFailed {
		t.Fatalf("execute phase after shutdown = %q/%q", phases[1].Phase, phases[1].Status)
	}
	if phases[1].Summary == nil || !strings.Contains(*phases[1].Summary, worker.ShutdownReason) {
		t.Fatalf("execute phase summary = %v, want contains %q", phases[1].Summary, worker.ShutdownReason)
	}

	events, _ := h.store.ListTaskEvents(bg, tsk.ID)
	counts := eventTypeCounts(events)
	if counts[domain.EventCycleFailed] != 1 {
		t.Fatalf("cycle_failed (aborted folds in) count = %d, want 1", counts[domain.EventCycleFailed])
	}
}

func TestWorker_NoDoubleCycleOnRedelivery(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "redeliver")

	br := newBlockingRunner()
	br.result = runner.NewResult(domain.PhaseStatusSucceeded, "", nil, "")

	// Direct in-test attempt to write a second cycle while one is
	// running surfaces ErrInvalidInput from the store guard. This
	// pins the substrate behaviour the worker relies on (edge case
	// from the plan: "no double cycle on redelivery").
	_, done := h.startWorker(ctx, br, worker.Options{})

	select {
	case req := <-br.starts:
		if req.TaskID != tsk.ID {
			t.Fatalf("first run task id = %s, want %s", req.TaskID, tsk.ID)
		}
	case <-time.After(pollTimeout):
		t.Fatal("timed out waiting for first runner Run")
	}

	_, err := h.store.StartCycle(context.Background(), store.StartCycleInput{
		TaskID: tsk.ID, TriggeredBy: domain.ActorAgent,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("second StartCycle err = %v, want ErrInvalidInput", err)
	}

	close(br.release)
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	cycles, _ := h.store.ListCyclesForTask(context.Background(), tsk.ID, 10)
	if len(cycles) != 1 {
		t.Fatalf("final cycle count = %d, want 1", len(cycles))
	}
}

func TestWorker_NilNotifierIsNoOp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "no-notifier")

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "", nil, ""))

	w := worker.NewWorker(h.store, h.queue, r, worker.Options{Notifier: nil})
	done := make(chan error, 1)
	go func() {
		done <- w.Run(ctx)
	}()

	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
}

// TestWorker_RunReturnsOnNilDeps fast-fails when the constructor was
// fed nil dependencies and Run is invoked anyway. Pins the contract
// for cmd/taskapi: don't go w.Run(ctx) until you've supplied real
// store/queue/runner instances.
func TestWorker_RunReturnsOnNilDeps(t *testing.T) {
	t.Parallel()
	w := &worker.Worker{}
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("expected error from nil-deps Run")
	}
}

// Sanity: NotifyReadyTask via Create + Update remains the supported
// enqueue path and the `BufferDepth` getter returns to zero after a
// happy run. Used by other tests indirectly; this one pins it.
func TestWorker_QueueDrainsAfterHappyRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "drain")
	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "", nil, ""))

	// Use atomic to keep the closure readonly to the linter.
	var ran atomic.Bool
	wrappedRunner := &funcRunner{
		name:    "wrap",
		version: "v1",
		run: func(ctx context.Context, req runner.Request) (runner.Result, error) {
			ran.Store(true)
			return r.Run(ctx, req)
		},
	}

	_, done := h.startWorker(ctx, wrappedRunner, worker.Options{})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}
	if !ran.Load() {
		t.Fatal("wrapped runner was not invoked")
	}
	if got := h.queue.BufferDepth(); got != 0 {
		t.Fatalf("queue depth after happy run = %d, want 0", got)
	}
}

type funcRunner struct {
	name, version string
	run           func(ctx context.Context, req runner.Request) (runner.Result, error)
}

func (f *funcRunner) Name() string    { return f.name }
func (f *funcRunner) Version() string { return f.version }
func (f *funcRunner) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	return f.run(ctx, req)
}
