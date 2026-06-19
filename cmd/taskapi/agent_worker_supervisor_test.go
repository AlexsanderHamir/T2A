package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker"
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy"
	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type supervisorTestRig struct {
	store *store.Store
	queue *agents.MemoryQueue
	hub   *handler.SSEHub
	sup   *agentworker.Supervisor
}

func newSupervisorTestRig(t *testing.T, ctx context.Context, probeFn func(ctx context.Context, id, bin string, timeout time.Duration) (string, string, error)) *supervisorTestRig {
	t.Helper()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)
	st.SetReadyTaskNotifier(q)
	hub := handler.NewSSEHub()
	sup := agentworker.New(ctx, st, q, hub)
	if probeFn != nil {
		sup.SetProbeForTest(probeFn)
	}
	sup.SetProbeBudgetForTest(200 * time.Millisecond)
	t.Cleanup(sup.Drain)
	return &supervisorTestRig{store: st, queue: q, hub: hub, sup: sup}
}

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
	if rig.sup.HasRunningInstance() {
		t.Errorf("supervisor spawned worker despite empty repo root")
	}
	if probeCalled {
		t.Error("probe called even though supervisor was idle on empty repo root")
	}
}

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
	rig.sup.SetProbeForTest(func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		probeCalled = true
		return "x", "", nil
	})

	if err := rig.sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if rig.sup.HasRunningInstance() {
		t.Errorf("supervisor spawned worker despite AgentPaused=true")
	}
	if probeCalled {
		t.Error("probe called for paused worker (should short-circuit before probe)")
	}
}

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
	if !rig.sup.HasRunningInstance() {
		t.Fatal("precondition: supervisor failed to spawn worker for valid config")
	}

	if _, err := rig.store.UpdateSettings(ctx, store.SettingsPatch{
		AgentPaused: ptrBool(true),
	}); err != nil {
		t.Fatalf("flip AgentPaused=true: %v", err)
	}
	if err := rig.sup.Reload(ctx); err != nil {
		t.Fatalf("Reload after pause: %v", err)
	}

	if rig.sup.HasRunningInstance() {
		t.Errorf("supervisor kept worker running despite AgentPaused=true after Reload")
	}
}

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
	if rig.sup.HasRunningInstance() {
		t.Errorf("supervisor spawned worker despite probe failure")
	}
}

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
	if !rig.sup.HasRunningInstance() {
		t.Fatal("supervisor failed to spawn worker for valid config")
	}
	version, ok := rig.sup.RunningInstanceRunnerVersion()
	if !ok || version != "test-version-1.2.3" {
		t.Errorf("runner version mismatch: got %q ok=%v", version, ok)
	}
}

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
	first := rig.sup.RunningInstanceIdentity()
	if first == 0 {
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
	second := rig.sup.RunningInstanceIdentity()
	if second == 0 {
		t.Fatal("reload dropped worker for valid config")
	}
	if second == first {
		t.Error("reload did not respawn worker on repo root change")
	}
	repoRoot, ok := rig.sup.RunningInstanceRepoRoot()
	if !ok || repoRoot != dirB {
		t.Errorf("worker repo root = %q ok=%v, want %q", repoRoot, ok, dirB)
	}
}

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
	first := rig.sup.RunningInstanceIdentity()

	if err := rig.sup.Reload(ctx); err != nil {
		t.Fatalf("reload (no-op): %v", err)
	}
	second := rig.sup.RunningInstanceIdentity()
	if first != second {
		t.Errorf("reload respawned worker without material change (first=%x second=%x)", first, second)
	}
}

func TestSupervisor_CancelCurrentRun_idleReturnsFalse(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if rig.sup.CancelCurrentRun() {
		t.Error("CancelCurrentRun() = true on a freshly constructed supervisor")
	}
}

func TestSupervisor_DrainAfterDrainIsNoOp(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	rig.sup.Drain()
	rig.sup.Drain()
}

func TestSupervisor_ConcurrentReloadIsSerialized(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		probeMu       sync.Mutex
		concurrentMax int32
		concurrentNow int32
		blockProbe    = make(chan struct{})
	)
	close(blockProbe)

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
		t.Errorf("Reload not serialized: %d probes ran concurrently (want <=1)", got)
	}
	if !rig.sup.HasRunningInstance() {
		t.Fatal("supervisor lost worker after concurrent Reload")
	}
	repoRoot, ok := rig.sup.RunningInstanceRepoRoot()
	if !ok || repoRoot != dirB {
		t.Errorf("final worker repo root = %q ok=%v, want %q", repoRoot, ok, dirB)
	}
}

func ptrBool(v bool) *bool           { return &v }
func ptrString(v string) *string     { return &v }
func ptrTime(v time.Time) *time.Time { return &v }

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
		{"queue-empty/some-scheduled", true, 1, policy.SchedulingIdleHintReason},
		{"queue-empty/many-scheduled", true, 42, policy.SchedulingIdleHintReason},
		{"queue-empty/no-scheduled", true, 0, ""},
		{"queue-empty/negative-defensive", true, -1, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := policy.DecideSchedulingIdleHint(c.queueEmpty, c.scheduledCount); got != c.want {
				t.Fatalf("DecideSchedulingIdleHint(%v, %d) = %q, want %q",
					c.queueEmpty, c.scheduledCount, got, c.want)
			}
		})
	}
}

func TestSupervisor_probeSchedulingHint_emitsAwaitingScheduledTask(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	future := time.Now().UTC().Add(time.Hour)
	if _, err := rig.store.Create(ctx, store.CreateTaskInput{
		Title:           "deferred",
		Priority:        domain.PriorityMedium,
		Status:          domain.StatusReady,
		PickupNotBefore: ptrTime(future),
	}, domain.ActorUser); err != nil {
		t.Fatalf("create deferred task: %v", err)
	}

	hint := rig.sup.ProbeSchedulingHintForTest(ctx)
	if hint != policy.SchedulingIdleHintReason {
		t.Fatalf("probeSchedulingHint = %q, want %q", hint, policy.SchedulingIdleHintReason)
	}
}

func TestSupervisor_probeSchedulingHint_silentWhenQueueHasReadyNow(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if _, err := rig.store.Create(ctx, store.CreateTaskInput{
		Title:    "ready-now",
		Priority: domain.PriorityMedium,
		Status:   domain.StatusReady,
	}, domain.ActorUser); err != nil {
		t.Fatalf("create ready-now task: %v", err)
	}
	future := time.Now().UTC().Add(time.Hour)
	if _, err := rig.store.Create(ctx, store.CreateTaskInput{
		Title:           "deferred",
		Priority:        domain.PriorityMedium,
		Status:          domain.StatusReady,
		PickupNotBefore: ptrTime(future),
	}, domain.ActorUser); err != nil {
		t.Fatalf("create deferred task: %v", err)
	}

	if hint := rig.sup.ProbeSchedulingHintForTest(ctx); hint != "" {
		t.Fatalf("probeSchedulingHint = %q, want \"\"", hint)
	}
}

func TestSupervisor_probeSchedulingHint_silentWhenNothingScheduled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, nil)
	if hint := rig.sup.ProbeSchedulingHintForTest(ctx); hint != "" {
		t.Fatalf("probeSchedulingHint = %q, want \"\"", hint)
	}
}

func TestSupervisor_buildVerifyRunner_returnsNilWhenUnconfigured(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		t.Fatal("probe must not be called when VerifyRunnerName is empty")
		return "", "", nil
	})
	r, status := rig.sup.BuildVerifyRunnerForTest(ctx, store.AppSettings{Runner: "cursor", VerifyRunnerName: ""})
	if r != nil || status != "" {
		t.Fatalf("buildVerifyRunner(unconfigured) = (%v, %q), want (nil, \"\")", r, status)
	}
}

func TestSupervisor_buildVerifyRunner_demotesOnProbeFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	probeCalls := 0
	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, id, _ string, _ time.Duration) (string, string, error) {
		probeCalls++
		return "", "", errors.New("verify binary not found")
	})
	r, status := rig.sup.BuildVerifyRunnerForTest(ctx, store.AppSettings{
		Runner:           "cursor",
		VerifyRunnerName: "claudecode",
	})
	if r != nil {
		t.Fatalf("expected nil runner on probe failure, got %v", r)
	}
	if status != "demoted_probe_failed" {
		t.Fatalf("status = %q, want demoted_probe_failed", status)
	}
	if probeCalls != 1 {
		t.Fatalf("probe calls = %d, want 1", probeCalls)
	}
}

func TestSupervisor_buildVerifyRunner_reuseExecuteRunnerWhenSameName(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rig := newSupervisorTestRig(t, ctx, func(_ context.Context, _, _ string, _ time.Duration) (string, string, error) {
		t.Fatal("probe must not be called when verify == execute")
		return "", "", nil
	})
	r, status := rig.sup.BuildVerifyRunnerForTest(ctx, store.AppSettings{
		Runner:           "cursor",
		VerifyRunnerName: "cursor",
	})
	if r != nil || status != "reuse_execute_runner" {
		t.Fatalf("buildVerifyRunner(same name) = (%v, %q), want (nil, reuse_execute_runner)", r, status)
	}
}

func TestInstanceMatchesSettings_restartsOnVerifyRunnerChange(t *testing.T) {
	t.Parallel()
	prev := &policy.InstanceSnapshot{
		Settings: store.AppSettings{
			Runner:            "cursor",
			VerifyRunnerName:  "claudecode",
			VerifyRunnerModel: "opus",
			RepoRoot:          "/x",
		},
	}
	matches := policy.InstanceMatchesSettings(prev, store.AppSettings{
		Runner:            "cursor",
		VerifyRunnerName:  "claudecode",
		VerifyRunnerModel: "sonnet-4.5",
		RepoRoot:          "/x",
	}, "")
	if matches {
		t.Fatal("expected restart trigger on VerifyRunnerModel change")
	}
	matches = policy.InstanceMatchesSettings(prev, store.AppSettings{
		Runner:            "cursor",
		VerifyRunnerName:  "cursor",
		VerifyRunnerModel: "opus",
		RepoRoot:          "/x",
	}, "")
	if matches {
		t.Fatal("expected restart trigger on VerifyRunnerName change")
	}
}
