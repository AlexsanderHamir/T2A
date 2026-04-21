package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/internal/taskapiconfig"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// run_agentworker.go owns the optional in-process agent worker
// supervisor: bounded ready-task queue, reconcile loop, runner-registry
// build, startup orphan sweep, runner probe, SSE notifier adapter, hot
// reload (called by PATCH /settings), in-flight cancel (called by
// POST /settings/cancel-current-run), and the shutdown-drain handle.
//
// Configuration source: pkgs/tasks/store.AppSettings (the singleton
// row managed via the SPA settings page). Env vars are no longer
// consulted — see docs/SETTINGS.md.

// shutdownGraceAfterRunTimeout is the headroom added to the configured
// per-run cap when waiting for Worker.Run to drain during shutdown.
// The extra slack covers the worker's own deferred best-effort writes
// (handleShutdownAfterRun) so they can land before the reconcile ctx
// and DB pool close. When the per-run cap is "no limit" (the default),
// drain falls back to draindNoLimitTimeout below.
const shutdownGraceAfterRunTimeout = 10 * time.Second

// drainNoLimitTimeout is the upper bound applied during shutdown when
// the operator picked "No limit" for the per-run cap. Without it a
// runaway run would block process exit indefinitely; the documented
// trade-off is that callers who want a true "wait forever" semantic
// must hit POST /settings/cancel-current-run before shutdown.
const drainNoLimitTimeout = 5 * time.Minute

// agentWorkerStartupSweepTimeout bounds the one-shot
// SweepOrphanRunningCycles call we run before each (re)start of the
// worker. Best-effort housekeeping for cycle/phase rows left in
// 'running' by a previous crash; if it can't finish in this budget we
// log and continue so a slow DB doesn't block startup indefinitely.
const agentWorkerStartupSweepTimeout = 30 * time.Second

// agentWorkerSupervisor owns the lifecycle of the in-process agent
// worker. Construct via newAgentWorkerSupervisor; drive with Start,
// Reload, CancelCurrentRun, Drain. Methods are safe for concurrent
// use; the lifecycle mutex serialises Start/Reload/Drain so we never
// race two worker goroutines.
//
// Lifecycle states and the methods that drive them:
//
//   - "not started" → Start() reads settings, builds runner, spawns
//     worker goroutine (or stays idle when WorkerEnabled is false /
//     RepoRoot is empty / runner probe fails).
//   - "running" → CancelCurrentRun() proxies to Worker.CancelCurrentRun;
//     Reload() rebuilds config under the lifecycle lock and respawns
//     when anything material changed.
//   - "draining" → Drain() cancels the worker ctx and waits for the
//     run loop with a bounded deadline.
//
// "Material change" means anything that affects how the worker would
// behave on the next dequeue: enabled flag, runner id, cursor binary,
// repo root, or the per-run cap. We always restart on a material
// change (V1) instead of trying to mutate a live worker; the dequeue
// loop is a single goroutine so the cost of a restart is one in-flight
// run finishing on the old config, then the new config taking over.
type agentWorkerSupervisor struct {
	parentCtx  context.Context
	store      *store.Store
	queue      *agents.MemoryQueue
	hub        *handler.SSEHub
	metrics    worker.RunMetrics
	probe      func(ctx context.Context, id, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error)
	probeBudge time.Duration

	// applyMu serializes Start / Reload end-to-end so the read-prev,
	// probe, build, spawn, swap-current sequence in applySettings runs
	// atomically with respect to concurrent calls. Without this, the
	// brief s.mu critical section that snapshots prev releases the
	// lock during the long-running probe + build + spawn — two
	// concurrent Reloads (e.g. two PATCH /settings requests landing
	// inside the probe budget) could both observe the same prev
	// pointer, both proceed to spawn fresh worker goroutines, and the
	// loser of the s.current swap would be leaked (its cancelCtx
	// never fires, its goroutine drains the ready queue forever
	// alongside the visible worker — see TestSupervisor_ConcurrentReloadIsSerialized).
	// applyMu is intentionally separate from s.mu so the in-flight
	// apply does not block CancelCurrentRun / Drain / Subscribe-style
	// short, hot-path s.mu critical sections; Drain still races the
	// in-flight apply via the s.drained flag (checked twice — at the
	// top under s.mu and again before swapping s.current — so a Drain
	// that lands mid-apply tears down the just-spawned worker and the
	// apply returns the "drained mid-start" error).
	applyMu sync.Mutex

	mu      sync.Mutex
	current *agentWorkerInstance
	drained bool
}

// agentWorkerInstance is one running worker incarnation. The
// supervisor swaps these out atomically on Reload; CancelCurrentRun
// proxies to whichever instance is current at call time.
type agentWorkerInstance struct {
	worker     *worker.Worker
	cancelCtx  context.CancelFunc
	doneCh     chan struct{}
	runTimeout time.Duration
	settings   store.AppSettings
	runner     runner.Runner
}

// effectiveSettingsLog is a struct-shaped projection of the settings
// snapshot used for startup / reload INFO logs so the operator can see
// the resolved values without having to hit GET /settings.
type effectiveSettingsLog struct {
	WorkerEnabled         bool
	AgentPaused           bool
	Runner                string
	RepoRoot              string
	CursorBin             string
	CursorModel           string
	MaxRunDurationSeconds int
	RunnerVersion         string
	Idle                  bool
	IdleReason            string
}

// AgentWorkerSupervisor is the public alias the HTTP handler uses to
// talk to the supervisor without taking a dependency on the unexported
// struct. Mirrors handler.AgentWorkerControl one-for-one so the
// supervisor satisfies the handler interface implicitly.
type AgentWorkerSupervisor interface {
	CancelCurrentRun() bool
	Reload(ctx context.Context) error
	ProbeRunner(ctx context.Context, runnerID, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error)
}

// newAgentWorkerSupervisor wires the supervisor with its dependencies.
// The supervisor does not start the worker; the caller invokes Start
// once after the rest of buildTaskAPIApp finishes (so the SSE hub +
// store are fully initialised before any settings_changed event is
// published).
func newAgentWorkerSupervisor(ctx context.Context, st *store.Store, q *agents.MemoryQueue, hub *handler.SSEHub) *agentWorkerSupervisor {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.newAgentWorkerSupervisor")
	return &agentWorkerSupervisor{
		parentCtx:  ctx,
		store:      st,
		queue:      q,
		hub:        hub,
		metrics:    taskapi.RegisterAgentWorkerMetrics(),
		probe:      registry.Probe,
		probeBudge: 5 * time.Second,
	}
}

// Start performs the first boot of the worker by delegating to Reload.
// Splitting Start as a thin wrapper keeps the lifecycle log differentiable
// (boot vs reload) without duplicating the (read settings → build →
// spawn) pipeline.
func (s *agentWorkerSupervisor) Start(ctx context.Context) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.Start")
	s.logPreFeatureCycleCount(ctx)
	return s.applySettings(ctx, "boot")
}

// logPreFeatureCycleCount emits ONE Info line at supervisor boot
// reporting how many terminal cycles predate the V2 runner/model
// attribution keys (see plan rollout_backfill / docs/TROUBLESHOOTING.md
// "Observability 'Runner & model' panel shows an empty model row").
//
// Best-effort: a transient store error degrades to a single Warn line
// and does NOT block startup — the count is operator information, not
// a precondition for serving traffic. Bounded by
// agentWorkerStartupSweepTimeout so a stalled DB cannot wedge boot.
//
// Called only from Start() so a `PATCH /settings` Reload does not
// re-emit the line on every settings change. The numbers age out as
// new cycles dominate the aggregates; there is no live re-count.
func (s *agentWorkerSupervisor) logPreFeatureCycleCount(ctx context.Context) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.logPreFeatureCycleCount")
	countCtx, cancel := context.WithTimeout(ctx, agentWorkerStartupSweepTimeout)
	defer cancel()
	counts, err := s.store.CountPreFeatureCycles(countCtx)
	if err != nil {
		slog.Warn("agent worker pre-feature cycle count skipped",
			"cmd", cmdName,
			"operation", "taskapi.agent_worker.pre_feature_count_err",
			"err", err)
		return
	}
	slog.Info("agent worker pre-feature cycles",
		"cmd", cmdName,
		"operation", "taskapi.agentWorkerSupervisor.startup.preFeatureCycleCount",
		"terminal_cycles_total", counts.Total,
		"missing_cursor_model_effective_key", counts.MissingKey,
		"empty_cursor_model_effective_value", counts.EmptyValue)
}

// Reload re-reads AppSettings and respawns the worker if anything
// material changed. Safe to call from any goroutine; serialised with
// Start and Drain via the lifecycle mutex. The HTTP handler invokes
// this after PATCH /settings persists.
func (s *agentWorkerSupervisor) Reload(ctx context.Context) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.Reload")
	return s.applySettings(ctx, "reload")
}

// ProbeRunner exposes the runner registry probe to the HTTP handler so
// the SPA "Test cursor binary" button can verify a binary path before
// Save. Reuses the same registry.Probe the supervisor uses on boot so
// the probe result the operator sees is identical to what would be
// observed at the next reload.
func (s *agentWorkerSupervisor) ProbeRunner(ctx context.Context, runnerID, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.ProbeRunner",
		"runner", runnerID, "binary", binaryPath)
	if timeout <= 0 {
		timeout = s.probeBudge
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.probe(probeCtx, runnerID, binaryPath, timeout)
}

// CancelCurrentRun cancels the in-flight runner.Run, if any. Returns
// true when there was a run to cancel. Mirrors Worker.CancelCurrentRun
// — see that method for the audit-trail invariant ("cancelled_by_operator"
// reason in the cycle_failed event).
func (s *agentWorkerSupervisor) CancelCurrentRun() bool {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.CancelCurrentRun")
	s.mu.Lock()
	inst := s.current
	s.mu.Unlock()
	if inst == nil || inst.worker == nil {
		return false
	}
	return inst.worker.CancelCurrentRun()
}

// Drain cancels the worker context and waits for Worker.Run to return,
// bounded by a deadline derived from the active per-run cap. Idempotent:
// repeated calls after the first are no-ops. The cmd/taskapi shutdown
// path calls Drain before closing the DB pool so any best-effort
// post-cancel writes (handleShutdownAfterRun) can land first.
func (s *agentWorkerSupervisor) Drain() {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.Drain")
	s.mu.Lock()
	if s.drained {
		s.mu.Unlock()
		return
	}
	s.drained = true
	inst := s.current
	s.current = nil
	s.mu.Unlock()
	stopWorkerInstance(inst, "shutdown")
}

// applySettings is the shared implementation behind Start and Reload.
// Reads settings, decides whether to spawn / replace / stop the worker,
// publishes a settings_changed SSE event so the SPA refreshes, and
// returns any hard error. Soft errors (idle reasons like an empty repo
// root or a failed cursor probe) are logged and result in the worker
// staying idle rather than failing the call — that way Reload can still
// "succeed" from the operator's perspective and the SPA shows the
// resolved status panel.
func (s *agentWorkerSupervisor) applySettings(ctx context.Context, phase string) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.applySettings",
		"phase", phase)
	// Serialize the entire apply pipeline. See applyMu doc on
	// agentWorkerSupervisor for why s.mu alone is insufficient
	// (the brief s.mu critical sections release before the long
	// probe/build/spawn section, opening a window for two
	// concurrent Reloads to both spawn workers and leak one).
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	cfg, err := s.store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("agent worker supervisor: read settings: %w", err)
	}

	s.mu.Lock()
	if s.drained {
		s.mu.Unlock()
		return errors.New("agent worker supervisor: already drained")
	}
	prev := s.current
	s.mu.Unlock()

	idle, reason := decideIdle(cfg)
	if idle {
		stopWorkerInstance(prev, "idle:"+reason)
		s.mu.Lock()
		if !s.drained {
			s.current = nil
		}
		s.mu.Unlock()
		s.logEffective(phase, effectiveSettingsLog{
			WorkerEnabled: cfg.WorkerEnabled, AgentPaused: cfg.AgentPaused,
			Runner:   cfg.Runner,
			RepoRoot: cfg.RepoRoot, CursorBin: cfg.CursorBin,
			CursorModel:           cfg.CursorModel,
			MaxRunDurationSeconds: cfg.MaxRunDurationSeconds,
			Idle:                  true, IdleReason: reason,
		})
		s.publishSettingsChanged()
		return nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, s.probeBudge)
	version, _, probeErr := s.probe(probeCtx, cfg.Runner, cfg.CursorBin, s.probeBudge)
	cancel()
	if probeErr != nil {
		stopWorkerInstance(prev, "probe_failed")
		s.mu.Lock()
		if !s.drained {
			s.current = nil
		}
		s.mu.Unlock()
		slog.Warn("agent worker probe failed; staying idle", "cmd", cmdName,
			"operation", "taskapi.agent_worker.probe_err", "phase", phase,
			"runner", cfg.Runner, "binary", cfg.CursorBin, "err", probeErr)
		s.logEffective(phase, effectiveSettingsLog{
			WorkerEnabled: cfg.WorkerEnabled, AgentPaused: cfg.AgentPaused,
			Runner:   cfg.Runner,
			RepoRoot: cfg.RepoRoot, CursorBin: cfg.CursorBin,
			CursorModel:           cfg.CursorModel,
			MaxRunDurationSeconds: cfg.MaxRunDurationSeconds,
			Idle:                  true, IdleReason: "probe_failed",
		})
		s.publishSettingsChanged()
		return nil
	}

	if prev != nil && instanceMatchesSettings(prev, cfg, version) {
		s.logEffective(phase, effectiveSettingsLog{
			WorkerEnabled: cfg.WorkerEnabled, AgentPaused: cfg.AgentPaused,
			Runner:   cfg.Runner,
			RepoRoot: cfg.RepoRoot, CursorBin: cfg.CursorBin,
			CursorModel:           cfg.CursorModel,
			MaxRunDurationSeconds: cfg.MaxRunDurationSeconds,
			RunnerVersion:         version,
			IdleReason:            s.probeSchedulingHint(ctx),
		})
		s.publishSettingsChanged()
		return nil
	}

	if err := s.runStartupSweep(ctx); err != nil {
		slog.Warn("agent worker startup sweep failed (continuing)",
			"cmd", cmdName, "operation", "taskapi.agent_worker.sweep_err",
			"err", err)
	}

	r, err := registry.Build(cfg.Runner, registry.BuildOptions{
		BinaryPath:  cfg.CursorBin,
		Version:     version,
		CursorModel: cfg.CursorModel,
	})
	if err != nil {
		stopWorkerInstance(prev, "build_failed")
		s.mu.Lock()
		if !s.drained {
			s.current = nil
		}
		s.mu.Unlock()
		return fmt.Errorf("agent worker build runner %q: %w", cfg.Runner, err)
	}

	runTimeout := time.Duration(cfg.MaxRunDurationSeconds) * time.Second
	notifier := newCycleChangeSSEAdapter(s.hub)
	w := worker.NewWorker(s.store, s.queue, r, worker.Options{
		RunTimeout: runTimeout,
		WorkingDir: cfg.RepoRoot,
		Notifier:   notifier,
		Metrics:    s.metrics,
	})

	workerCtx, cancelWorker := context.WithCancel(s.parentCtx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Run(workerCtx); err != nil {
			slog.Error("agent worker exited with error", "cmd", cmdName,
				"operation", "taskapi.agent_worker.exit_err", "err", err)
		}
	}()

	next := &agentWorkerInstance{
		worker: w, cancelCtx: cancelWorker, doneCh: done,
		runTimeout: runTimeout, settings: cfg, runner: r,
	}

	s.mu.Lock()
	if s.drained {
		s.mu.Unlock()
		cancelWorker()
		<-done
		return errors.New("agent worker supervisor: drained mid-start")
	}
	s.current = next
	s.mu.Unlock()

	stopWorkerInstance(prev, "reload")

	s.logEffective(phase, effectiveSettingsLog{
		WorkerEnabled: cfg.WorkerEnabled, AgentPaused: cfg.AgentPaused,
		Runner:   cfg.Runner,
		RepoRoot: cfg.RepoRoot, CursorBin: cfg.CursorBin,
		CursorModel:           cfg.CursorModel,
		MaxRunDurationSeconds: cfg.MaxRunDurationSeconds,
		RunnerVersion:         version,
		IdleReason:            s.probeSchedulingHint(ctx),
	})
	s.publishSettingsChanged()
	return nil
}

// probeSchedulingHint runs the bounded queue+stats probes used by
// decideSchedulingIdleHint. Both calls are best-effort: any error
// (DB blip, context cancel, etc.) returns "" so the effective-config
// log line stays useful even when the diagnostic can't be computed.
// Bounded by a tiny budget — the probe must not delay supervisor
// state transitions even when the DB is slow.
func (s *agentWorkerSupervisor) probeSchedulingHint(ctx context.Context) string {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.probeSchedulingHint")
	if s.store == nil {
		return ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	candidates, err := s.store.ListReadyTaskQueueCandidates(probeCtx, 1, nil)
	if err != nil {
		slog.Debug("scheduling hint: queue probe failed",
			"cmd", cmdName, "operation", "taskapi.agent_worker.scheduling_hint_queue_err",
			"err", err)
		return ""
	}
	if len(candidates) > 0 {
		// Queue is non-empty: by definition NOT awaiting a
		// scheduled task. Skip the stats round-trip.
		return ""
	}
	stats, err := s.store.TaskStats(probeCtx)
	if err != nil {
		slog.Debug("scheduling hint: stats probe failed",
			"cmd", cmdName, "operation", "taskapi.agent_worker.scheduling_hint_stats_err",
			"err", err)
		return ""
	}
	return decideSchedulingIdleHint(true, stats.Scheduled)
}

func (s *agentWorkerSupervisor) runStartupSweep(ctx context.Context) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.runStartupSweep")
	sweepCtx, cancel := context.WithTimeout(ctx, agentWorkerStartupSweepTimeout)
	defer cancel()
	res, err := worker.SweepOrphanRunningCycles(sweepCtx, s.store)
	if err != nil {
		return err
	}
	slog.Info("agent worker startup sweep ok", "cmd", cmdName,
		"operation", "taskapi.agent_worker.sweep_ok",
		"cycles_aborted", res.CyclesAborted, "phases_failed", res.PhasesFailed,
		"tasks_failed", res.TasksFailed)
	return nil
}

func (s *agentWorkerSupervisor) logEffective(phase string, eff effectiveSettingsLog) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.logEffective",
		"phase", phase)
	slog.Info("agent worker effective config", "cmd", cmdName, "operation", "taskapi.agent_worker",
		"phase", phase,
		"enabled", eff.WorkerEnabled, "paused", eff.AgentPaused,
		"idle", eff.Idle, "idle_reason", eff.IdleReason,
		"runner", eff.Runner, "runner_version", eff.RunnerVersion,
		"repo_root", eff.RepoRoot, "cursor_bin", eff.CursorBin,
		"cursor_model", eff.CursorModel,
		"max_run_duration_sec", eff.MaxRunDurationSeconds)
}

// publishSettingsChanged fires a settings_changed SSE so any open SPA
// settings page refreshes its view. Gate 3 wires the corresponding
// handler.TaskChangeType constant; this call is a no-op until then
// (hub treats unknown types as INFO log + drop).
func (s *agentWorkerSupervisor) publishSettingsChanged() {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.publishSettingsChanged")
	if s.hub == nil {
		return
	}
	s.hub.Publish(handler.TaskChangeEvent{Type: handler.SettingsChanged})
}

// SchedulingIdleHintReason is the diagnostic idle reason emitted when
// the worker is fully configured and could run, but the ready queue
// is empty *only because* every ready task is currently deferred via
// `pickup_not_before > now`. This is intentionally not returned by
// `decideIdle` (and therefore does NOT prevent the worker from
// spawning) because the schedule horizon could expire on the next
// reconcile tick and the worker must already be live to pick the
// task up — see docs/SCHEDULING.md "the two queues" section. The
// supervisor surfaces this as `IdleReason` on `effectiveSettingsLog`
// alongside `Idle=false` so operators reading logs and the
// observability page see the same explanation for "0 ready, 12
// scheduled" without conflating it with the genuine
// disabled/paused/probe-failed idle states.
const SchedulingIdleHintReason = "awaiting_scheduled_task"

// decideSchedulingIdleHint reports the diagnostic hint described
// above. It is a *runtime* probe (not part of decideIdle) so the
// supervisor can surface "intentionally deferred" without idling the
// worker process. Both probe arguments are bounded: queue probe asks
// for one row only, stats probe is the same single-COUNT pass that
// /tasks/stats already pays for. Errors degrade silently to "" so a
// transient DB hiccup does not poison the effective-config log line
// (the rest of the log is best-effort observability anyway).
//
// queueEmpty must report whether ListReadyTaskQueueCandidates returns
// zero rows (i.e. no row currently satisfies `status='ready' AND
// (pickup_not_before IS NULL OR pickup_not_before <= now)`).
//
// scheduledCount must report stats.Scheduled — the number of
// `status='ready' AND pickup_not_before > now` rows.
func decideSchedulingIdleHint(queueEmpty bool, scheduledCount int64) string {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.decideSchedulingIdleHint",
		"queue_empty", queueEmpty, "scheduled_count", scheduledCount)
	if queueEmpty && scheduledCount > 0 {
		return SchedulingIdleHintReason
	}
	return ""
}

// decideIdle reports whether the worker should stay idle given the
// effective settings. Returns (false, "") when the worker should run.
// Centralised so the boot, reload, and (future) HTTP probe paths agree
// on what counts as "configured enough to run".
//
// NOTE: this is intentionally a config-only check. The runtime
// "awaiting_scheduled_task" hint lives in decideSchedulingIdleHint
// because it must NOT prevent the worker from spawning — see that
// function's doc.
func decideIdle(cfg store.AppSettings) (bool, string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.decideIdle",
		"enabled", cfg.WorkerEnabled, "paused", cfg.AgentPaused,
		"repo_root", cfg.RepoRoot)
	if !cfg.WorkerEnabled {
		return true, "disabled_by_settings"
	}
	// Operator-facing soft pause. Distinct reason from
	// disabled_by_settings so the observability page can render
	// "Paused" (amber) vs "Disabled" (red/grey) accurately. Pause is
	// checked AFTER WorkerEnabled because a fully-disabled worker
	// dominates a paused-but-otherwise-running worker; either keeps
	// us idle, but disabled is the stronger signal.
	if cfg.AgentPaused {
		return true, "paused_by_operator"
	}
	if cfg.RepoRoot == "" {
		return true, "repo_root_not_configured"
	}
	if err := assertWorkingDirExists(cfg.RepoRoot); err != nil {
		slog.Warn("agent worker repo root not usable; staying idle",
			"cmd", cmdName, "operation", "taskapi.agent_worker.repo_root_err",
			"path", cfg.RepoRoot, "err", err)
		return true, "repo_root_invalid"
	}
	return false, ""
}

// instanceMatchesSettings reports whether the running worker already
// matches the desired settings. Used by Reload to skip pointless
// restarts when an operator hits Save without changing anything (or
// when the patch only touched fields the worker doesn't care about).
func instanceMatchesSettings(inst *agentWorkerInstance, cfg store.AppSettings, version string) bool {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.instanceMatchesSettings")
	if inst == nil {
		return false
	}
	if inst.settings.Runner != cfg.Runner {
		return false
	}
	if inst.settings.CursorBin != cfg.CursorBin {
		return false
	}
	if inst.settings.CursorModel != cfg.CursorModel {
		return false
	}
	if inst.settings.RepoRoot != cfg.RepoRoot {
		return false
	}
	if inst.settings.MaxRunDurationSeconds != cfg.MaxRunDurationSeconds {
		return false
	}
	if !inst.settings.WorkerEnabled {
		return false
	}
	// A pause flip changes effective state (idle vs running) even
	// though all other fields match — return false so applySettings
	// reaches its idle branch and stops the running instance.
	if inst.settings.AgentPaused != cfg.AgentPaused {
		return false
	}
	if inst.runner != nil && inst.runner.Version() != version {
		return false
	}
	return true
}

// stopWorkerInstance cancels and drains a single worker incarnation
// with a bounded deadline. Safe with a nil instance (no-op). The
// reason label feeds the structured log so post-mortems can tell apart
// reload, shutdown, and idle stops.
func stopWorkerInstance(inst *agentWorkerInstance, reason string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.stopWorkerInstance",
		"reason", reason)
	if inst == nil || inst.cancelCtx == nil {
		return
	}
	inst.cancelCtx()
	deadline := inst.runTimeout + shutdownGraceAfterRunTimeout
	if inst.runTimeout <= 0 {
		deadline = drainNoLimitTimeout
	}
	select {
	case <-inst.doneCh:
		slog.Info("agent worker instance stopped", "cmd", cmdName,
			"operation", "taskapi.agent_worker.stop", "reason", reason)
	case <-time.After(deadline):
		slog.Warn("agent worker instance drain timeout", "cmd", cmdName,
			"operation", "taskapi.agent_worker.stop_timeout",
			"reason", reason, "deadline", deadline.String())
	}
}

// startReadyTaskAgents wires the bounded ready-task queue, pickup wake
// scheduler, reconcile loop, and the agent worker supervisor. The
// reconcile loop is always on; the worker is gated on
// AppSettings.WorkerEnabled and dependencies (repo root, runner probe).
// The returned cancel func stops pickup wake and tears down the reconcile
// goroutine; the supervisor owns the worker lifecycle and exposes Drain
// for shutdown.
func startReadyTaskAgents(ctx context.Context, taskStore *store.Store, hub *handler.SSEHub) (context.CancelFunc, *agents.MemoryQueue, *agentWorkerSupervisor, error) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.startReadyTaskAgents")
	qcap := taskapiconfig.UserTaskAgentQueueCap()
	agentQueue := agents.NewMemoryQueue(qcap)
	taskStore.SetReadyTaskNotifier(agentQueue)
	pickupWake := agents.NewPickupWakeScheduler(taskStore, agentQueue)
	taskStore.SetPickupWake(pickupWake)
	if err := pickupWake.Hydrate(ctx); err != nil {
		return nil, nil, nil, err
	}
	iv := agents.ReconcileTickInterval
	slog.Info("ready task agent queue", "cmd", cmdName, "operation", "taskapi.agent_queue", "cap", qcap)
	slog.Info("ready task agent reconcile", "cmd", cmdName, "operation", "taskapi.agent_reconcile",
		"tick_interval", iv.String())

	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv)

	sup := newAgentWorkerSupervisor(ctx, taskStore, agentQueue, hub)
	if err := sup.Start(ctx); err != nil {
		pickupWake.Stop()
		reconcileCancel()
		return nil, nil, nil, err
	}
	stopAgents := func() {
		pickupWake.Stop()
		reconcileCancel()
	}
	return stopAgents, agentQueue, sup, nil
}

// assertWorkingDirExists is the fail-fast guard for AppSettings.RepoRoot.
// Returns an error when the path is missing or not a directory; the
// supervisor logs the error and stays idle.
func assertWorkingDirExists(dir string) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.assertWorkingDirExists",
		"dir", dir)
	if dir == "" {
		return errors.New("working directory is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", dir)
	}
	return nil
}

// cycleChangeSSEAdapter implements worker.CycleChangeNotifier on top
// of the existing handler.SSEHub. The TaskCycleChanged event type and
// the SPA cache invalidation hook are pinned by docs/API-SSE.md and
// docs/EXECUTION-CYCLES.md.
type cycleChangeSSEAdapter struct {
	hub *handler.SSEHub
}

func newCycleChangeSSEAdapter(hub *handler.SSEHub) *cycleChangeSSEAdapter {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.newCycleChangeSSEAdapter")
	return &cycleChangeSSEAdapter{hub: hub}
}

// PublishCycleChange satisfies worker.CycleChangeNotifier. Nil hub or
// blank ids are no-ops so the adapter is safe to wire even before the
// SSE listener is fully attached.
func (a *cycleChangeSSEAdapter) PublishCycleChange(taskID, cycleID string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.cycleChangeSSEAdapter.PublishCycleChange",
		"task_id", taskID, "cycle_id", cycleID)
	if a == nil || a.hub == nil || taskID == "" {
		return
	}
	a.hub.Publish(handler.TaskChangeEvent{
		Type:    handler.TaskCycleChanged,
		ID:      taskID,
		CycleID: cycleID,
	})
}
