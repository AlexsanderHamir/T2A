package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// supervisorTestRig wires a real store + queue + hub against an
// in-memory SQLite DB and lets each test inject a stub probe so the
// supervisor never spawns a real cursor binary.
type supervisorTestRig struct {
	store *store.Store
	queue *agents.MemoryQueue
	hub   *handler.SSEHub
	sup   *agentWorkerSupervisor
}

func newSupervisorTestRig(t *testing.T, ctx context.Context, probeFn func(ctx context.Context, id, bin string, timeout time.Duration) (string, string, error)) *supervisorTestRig {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)
	st.SetReadyTaskNotifier(q)
	hub := handler.NewSSEHub()
	sup := newAgentWorkerSupervisor(ctx, st, q, hub)
	if probeFn != nil {
		sup.probe = probeFn
	}
	sup.probeBudge = 200 * time.Millisecond
	t.Cleanup(sup.Drain)
	return &supervisorTestRig{store: st, queue: q, hub: hub, sup: sup}
}

// TestSupervisor_StaysIdleWhenRepoRootEmpty pins the documented "no
// repo configured -> worker idle" branch. The probe must NOT be called
// (that's the whole point of the early idle return: an operator who
// hasn't picked a repo root yet won't see misleading "cursor not
// installed" errors).
func TestSupervisor_StaysIdleWhenRepoRootEmpty(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	probeCalled := false
	probe := func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		probeCalled = true
		return "should-not-call", "", nil
	}
	rig := newSupervisorTestRig(t, ctx, probe)

	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur != nil {
		t.Errorf("supervisor spawned worker despite empty repo root")
	}
	if probeCalled {
		t.Error("probe called even though supervisor was idle on empty repo root")
	}
}

// TestSupervisor_StaysIdleWhenWorkerDisabled confirms WorkerEnabled=false
// short-circuits before the probe — same fail-fast logic as repo root
// (no point spending the probe budget on a runner that won't run).
func TestSupervisor_StaysIdleWhenWorkerDisabled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		WorkerEnabled: ptrBool(false),
		RepoRoot:      ptrString(t.TempDir()),
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	probeCalled := false
	rig.sup.probe = func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		probeCalled = true
		return "x", "", nil
	}

	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur != nil {
		t.Errorf("supervisor spawned worker despite WorkerEnabled=false")
	}
	if probeCalled {
		t.Error("probe called for disabled worker")
	}
}

// TestSupervisor_StaysIdleWhenAgentPaused mirrors the WorkerEnabled
// idle path for the soft-pause flag. The pause flag is the
// operator-facing "stop dequeuing for now" toggle exposed in the SPA
// header chip; it must short-circuit before the probe so a paused
// process doesn't burn its probe budget on every Reload.
func TestSupervisor_StaysIdleWhenAgentPaused(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		AgentPaused: ptrBool(true),
		RepoRoot:    ptrString(t.TempDir()),
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	probeCalled := false
	rig.sup.probe = func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		probeCalled = true
		return "x", "", nil
	}

	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur != nil {
		t.Errorf("supervisor spawned worker despite AgentPaused=true")
	}
	if probeCalled {
		t.Error("probe called for paused worker (should short-circuit before probe)")
	}
}

// TestSupervisor_ReloadStopsRunningWorkerOnPause covers the live-pause
// path: a worker that was happily running must stop on the next
// Reload after AgentPaused flips to true. instanceMatchesSettings has
// to honor the flag for this to work — without that, Reload would
// observe "all the runner-shaped fields match" and skip the restart,
// leaving the worker dequeuing despite the operator's pause click.
func TestSupervisor_ReloadStopsRunningWorkerOnPause(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		return "test-version-1.2.3", "", nil
	})
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(t.TempDir()),
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rig.sup.mu.Lock()
	if rig.sup.current == nil {
		rig.sup.mu.Unlock()
		t.Fatal("precondition: supervisor failed to spawn worker for valid config")
	}
	rig.sup.mu.Unlock()

	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		AgentPaused: ptrBool(true),
	}); err != nil {
		t.Fatalf("flip AgentPaused=true: %v", err)
	}
	if err := rig.sup.Reload(ctx); err != nil {
		t.Fatalf("Reload after pause: %v", err)
	}

	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur != nil {
		t.Errorf("supervisor kept worker running despite AgentPaused=true after Reload")
	}
}

// TestSupervisor_ProbeFailureKeepsIdle ensures a failing probe (e.g.
// cursor not installed) does not crash boot — instead the supervisor
// stays idle and Start returns nil. This is what makes the "configure
// later through the SPA" UX work.
func TestSupervisor_ProbeFailureKeepsIdle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		return "", "", errors.New("cursor not installed")
	})
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(t.TempDir()),
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start should not surface probe failure: %v", err)
	}
	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur != nil {
		t.Errorf("supervisor spawned worker despite probe failure")
	}
}

// TestSupervisor_StartsWorkerWhenConfigured exercises the happy path:
// repo root + worker enabled + probe ok = a running worker. Pins the
// invariant that current is non-nil after Start so the SPA "Status"
// panel can show the runner version sourced from the live worker.
func TestSupervisor_StartsWorkerWhenConfigured(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		return "test-version-1.2.3", "", nil
	})
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(t.TempDir()),
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur == nil {
		t.Fatal("supervisor failed to spawn worker for valid config")
	}
	if cur.runner == nil || cur.runner.Version() != "test-version-1.2.3" {
		t.Errorf("runner version mismatch: got %v", cur.runner)
	}
}

// TestSupervisor_ReloadRespawnsOnRepoRootChange covers the hot-reload
// path that makes the SPA Save button feel instant: changing the repo
// root tears down the old worker and spawns a new one with the new
// settings. Without the respawn the SPA would have to instruct the
// operator to restart the process.
func TestSupervisor_ReloadRespawnsOnRepoRootChange(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		return "v1", "", nil
	})
	dirA := t.TempDir()
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(dirA),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	rig.sup.mu.Lock()
	first := rig.sup.current
	rig.sup.mu.Unlock()
	if first == nil {
		t.Fatal("first start did not spawn worker")
	}

	dirB := t.TempDir()
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(dirB),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := rig.sup.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}
	rig.sup.mu.Lock()
	second := rig.sup.current
	rig.sup.mu.Unlock()
	if second == nil {
		t.Fatal("reload dropped worker for valid config")
	}
	if second == first {
		t.Error("reload did not respawn worker on repo root change")
	}
	if second.settings.RepoRoot != dirB {
		t.Errorf("worker settings.RepoRoot = %q, want %q", second.settings.RepoRoot, dirB)
	}
}

// TestSupervisor_ReloadSkipsRespawnOnNoMaterialChange protects against
// gratuitous worker churn: if PATCH /settings lands without changing
// any field the supervisor cares about, the in-flight worker stays.
// Without this check, every Save click would interrupt an in-flight
// run for no reason.
func TestSupervisor_ReloadSkipsRespawnOnNoMaterialChange(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		return "v1", "", nil
	})
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(t.TempDir()),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	rig.sup.mu.Lock()
	first := rig.sup.current
	rig.sup.mu.Unlock()

	if err := rig.sup.Reload(ctx); err != nil {
		t.Fatalf("reload (no-op): %v", err)
	}
	rig.sup.mu.Lock()
	second := rig.sup.current
	rig.sup.mu.Unlock()
	if first != second {
		t.Errorf("reload respawned worker without material change (first=%p second=%p)", first, second)
	}
}

// TestSupervisor_CancelCurrentRun_idleReturnsFalse pins the documented
// "no run in flight = no-op" semantic so the HTTP handler can call
// CancelCurrentRun unconditionally and use the return value to decide
// whether to fan out an SSE event.
func TestSupervisor_CancelCurrentRun_idleReturnsFalse(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if rig.sup.CancelCurrentRun() {
		t.Error("CancelCurrentRun() = true on a freshly constructed supervisor")
	}
}

// TestSupervisor_DrainAfterDrainIsNoOp covers the documented idempotent
// shutdown: signal handlers fire, Drain runs, and any deferred cleanup
// that calls Drain again should be a safe no-op rather than a panic.
func TestSupervisor_DrainAfterDrainIsNoOp(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	rig.sup.Drain()
	rig.sup.Drain()
}

// TestSupervisor_ConcurrentReloadIsSerialized pins the supervisor's
// Reload contract: concurrent calls (e.g. two PATCH /settings requests
// landing within the probe budget) must be serialized end-to-end so
// they observe each other's effects on s.current. Before the
// applyMu fix, applySettings released the lifecycle mutex during the
// long-running probe + build + spawn section: two Reloads could both
// snapshot the same prev pointer, both proceed to spawn fresh worker
// goroutines, and the second one would overwrite s.current — leaking
// the worker the first call had just spawned (its cancelCtx never
// fires, its goroutine drains the ready queue forever). The defect
// surfaces as "two probes ran concurrently": probe is the slowest
// observable step and a fast-and-loose proxy for the spawn race
// because both paths must traverse it before the supervisor mutates
// s.current. We also assert that applyMu's contract is end-to-end —
// the second Reload's probe MUST run after the first Reload has
// already updated s.current, so it can observe the new prev (set by
// instanceMatchesSettings to a no-op respawn or correctly tear down
// the previous-previous worker via stopWorkerInstance).
func TestSupervisor_ConcurrentReloadIsSerialized(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		probeMu       sync.Mutex
		concurrentMax int32
		concurrentNow int32
		// blockProbe gate is replaced under probeMu when we want
		// to switch the probe behaviour from fast (Start) to slow
		// (concurrent Reload).
		blockProbe = make(chan struct{})
	)
	close(blockProbe) // Start phase: no blocking.

	getGate := func() chan struct{} {
		probeMu.Lock()
		defer probeMu.Unlock()
		return blockProbe
	}

	probe := func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		n := atomic.AddInt32(&concurrentNow, 1)
		defer atomic.AddInt32(&concurrentNow, -1)
		for {
			cur := atomic.LoadInt32(&concurrentMax)
			if n <= cur || atomic.CompareAndSwapInt32(&concurrentMax, cur, n) {
				break
			}
		}
		<-getGate()
		return "v1", "", nil
	}

	rig := newSupervisorTestRig(t, ctx, probe)
	dirA := t.TempDir()
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(dirA),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if atomic.LoadInt32(&concurrentMax) != 1 {
		t.Fatalf("Start should have probed exactly once (concurrentMax=%d)", concurrentMax)
	}

	// Switch to a blocking gate so concurrent Reloads pile up
	// inside the probe and we can measure overlap.
	probeMu.Lock()
	blockProbe = make(chan struct{})
	probeMu.Unlock()
	atomic.StoreInt32(&concurrentMax, 0)
	atomic.StoreInt32(&concurrentNow, 0)

	dirB := t.TempDir()
	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		RepoRoot: ptrString(dirB),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- rig.sup.Reload(ctx)
		}()
	}
	// Give both goroutines time to enter the probe (or be blocked
	// at applyMu, depending on the fix). 200ms is comfortably above
	// the 5ms scheduling jitter floor on Windows runners and well
	// below the t.Cleanup -> Drain bound.
	time.Sleep(200 * time.Millisecond)
	probeMu.Lock()
	close(blockProbe)
	probeMu.Unlock()
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("Reload failed: %v", e)
		}
	}

	if got := atomic.LoadInt32(&concurrentMax); got > 1 {
		t.Errorf("Reload not serialized: %d probes ran concurrently (want <=1) — concurrent applySettings can leak worker goroutines by overwriting s.current after both probes return", got)
	}
	rig.sup.mu.Lock()
	cur := rig.sup.current
	rig.sup.mu.Unlock()
	if cur == nil {
		t.Fatal("supervisor lost worker after concurrent Reload")
	}
	if cur.settings.RepoRoot != dirB {
		t.Errorf("final worker settings.RepoRoot = %q, want %q (last write wins)", cur.settings.RepoRoot, dirB)
	}
}

func ptrBool(v bool) *bool           { return &v }
func ptrString(v string) *string     { return &v }
func ptrTime(v time.Time) *time.Time { return &v }

// TestDecideSchedulingIdleHint_unitTable pins the truth table for the
// pure helper. The hint must fire ONLY when the queue is empty AND
// there is at least one ready+future row — every other combination
// (queue non-empty, no scheduled rows, both empty) returns "" so the
// supervisor's effective-config log line stays uncluttered. This is
// the operator-visible "0 ready, N scheduled" vs "0 ready, 0
// scheduled" distinction promised in docs/SCHEDULING.md.
func TestDecideSchedulingIdleHint_unitTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		queueEmpty     bool
		scheduledCount int64
		want           string
	}{
		{"queue-non-empty/some-scheduled", false, 5, ""},
		{"queue-non-empty/no-scheduled", false, 0, ""},
		{"queue-empty/some-scheduled", true, 1, SchedulingIdleHintReason},
		{"queue-empty/many-scheduled", true, 42, SchedulingIdleHintReason},
		{"queue-empty/no-scheduled", true, 0, ""},
		{"queue-empty/negative-defensive", true, -1, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decideSchedulingIdleHint(c.queueEmpty, c.scheduledCount); got != c.want {
				t.Fatalf("decideSchedulingIdleHint(%v, %d) = %q, want %q",
					c.queueEmpty, c.scheduledCount, got, c.want)
			}
		})
	}
}

// TestSupervisor_probeSchedulingHint_emitsAwaitingScheduledTask is the
// integration counterpart: drive the real supervisor against an
// in-memory store seeded so the queue is empty (no ready+now rows)
// but the scheduled count is positive (one ready+future row), then
// assert probeSchedulingHint returns the documented reason. The
// alternate seedings (queue non-empty / nothing scheduled) are
// covered by the unit-table test above; this test pins the wire-up
// (store -> probe -> hint).
func TestSupervisor_probeSchedulingHint_emitsAwaitingScheduledTask(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	// One ready task scheduled an hour into the future.
	// ListReadyTaskQueueCandidates excludes it (filter:
	// pickup_not_before <= now). stats.Scheduled counts it
	// (predicate: pickup_not_before > now).
	future := time.Now().UTC().Add(time.Hour)
	if _, err := rig.store.Create(ctx, store.CreateTaskInput{
		Title:           "deferred",
		Priority:        domain.PriorityMedium,
		Status:          domain.StatusReady,
		PickupNotBefore: ptrTime(future),
	}, domain.ActorUser); err != nil {
		t.Fatalf("create deferred task: %v", err)
	}

	hint := rig.sup.probeSchedulingHint(ctx)
	if hint != SchedulingIdleHintReason {
		t.Fatalf("probeSchedulingHint = %q, want %q (ready+future task should fire the hint)",
			hint, SchedulingIdleHintReason)
	}
}

// TestSupervisor_probeSchedulingHint_silentWhenQueueHasReadyNow pins
// the negative case: when at least one ready task is dequeue-eligible
// right now, the supervisor must NOT surface "awaiting_scheduled_task"
// even if other ready rows are scheduled for later. Otherwise the log
// line would mislead operators into thinking the worker is idle when
// it actually has work pending.
func TestSupervisor_probeSchedulingHint_silentWhenQueueHasReadyNow(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	// One ready+now (no schedule) task → queue non-empty.
	if _, err := rig.store.Create(ctx, store.CreateTaskInput{
		Title:    "ready-now",
		Priority: domain.PriorityMedium,
		Status:   domain.StatusReady,
	}, domain.ActorUser); err != nil {
		t.Fatalf("create ready-now task: %v", err)
	}
	// Plus one ready+future task → stats.Scheduled = 1.
	future := time.Now().UTC().Add(time.Hour)
	if _, err := rig.store.Create(ctx, store.CreateTaskInput{
		Title:           "deferred",
		Priority:        domain.PriorityMedium,
		Status:          domain.StatusReady,
		PickupNotBefore: ptrTime(future),
	}, domain.ActorUser); err != nil {
		t.Fatalf("create deferred task: %v", err)
	}

	if hint := rig.sup.probeSchedulingHint(ctx); hint != "" {
		t.Fatalf("probeSchedulingHint = %q, want \"\" (queue non-empty must suppress the hint)", hint)
	}
}

// TestSupervisor_probeSchedulingHint_silentWhenNothingScheduled pins
// the truly-idle case: empty database (no ready, no scheduled). The
// supervisor must not invent a reason — the absence of work is just
// "0 ready, 0 scheduled", not "awaiting_scheduled_task".
func TestSupervisor_probeSchedulingHint_silentWhenNothingScheduled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if hint := rig.sup.probeSchedulingHint(ctx); hint != "" {
		t.Fatalf("probeSchedulingHint = %q, want \"\" (empty DB must not fire the hint)", hint)
	}
}
