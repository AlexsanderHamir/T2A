package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
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

func newSupervisorTestRig(t *testing.T, ctx context.Context, probeFn func(ctx context.Context, id, bin string, timeout time.Duration) (string, error)) *supervisorTestRig {
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
	probe := func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
		probeCalled = true
		return "should-not-call", nil
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
	rig.sup.probe = func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
		probeCalled = true
		return "x", nil
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

// TestSupervisor_ProbeFailureKeepsIdle ensures a failing probe (e.g.
// cursor not installed) does not crash boot — instead the supervisor
// stays idle and Start returns nil. This is what makes the "configure
// later through the SPA" UX work.
func TestSupervisor_ProbeFailureKeepsIdle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
		return "", errors.New("cursor not installed")
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

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
		return "test-version-1.2.3", nil
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

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
		return "v1", nil
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

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
		return "v1", nil
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

func ptrBool(v bool) *bool       { return &v }
func ptrString(v string) *string { return &v }
