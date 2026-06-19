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
	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy"
	"github.com/AlexsanderHamir/T2A/internal/taskapiconfig"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
	_ "github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry/all"
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
// consulted — see docs/configuration.md.

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

const agentRunProgressMinInterval = 750 * time.Millisecond
const agentRunProgressThrottleEntries = 512

// agentWorkerSupervisor owns the lifecycle of the in-process agent
// worker. Construct via newAgentWorkerSupervisor; drive with Start,
// Reload, CancelCurrentRun, Drain. Methods are safe for concurrent
// use; the lifecycle mutex serialises Start/Reload/Drain so we never
// race two worker goroutines.
//
// Lifecycle states and the methods that drive them:
//
//   - "not started" → Start() reads settings, builds runner, spawns
//     worker goroutine (or stays idle when AgentPaused is true /
//     RepoRoot is empty / runner probe fails).
//   - "running" → CancelCurrentRun() proxies to Worker.CancelCurrentRun;
//     Reload() rebuilds config under the lifecycle lock and respawns
//     when anything material changed.
//   - "draining" → Drain() cancels the worker ctx and waits for the
//     run loop with a bounded deadline.
//
// "Material change" means anything that affects how the worker would
// behave on the next dequeue: pause flag, runner id, cursor binary,
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
	// verifyRunner is the optional adversarial verify-pass runner. Nil
	// means "reuse execute runner" — either VerifyRunnerName was empty
	// in app_settings or the supervisor's build/probe failed and
	// demoted with a warn. instanceMatchesSettings inspects this so a
	// settings-page edit to VerifyRunnerName/Model triggers a restart.
	verifyRunner runner.Runner
}

// effectiveSettingsLog is a struct-shaped projection of the settings
// snapshot used for startup / reload INFO logs so the operator can see
// the resolved values without having to hit GET /settings.
type effectiveSettingsLog struct {
	AgentPaused           bool
	Runner                string
	RepoRoot              string
	CursorBin             string
	CursorModel           string
	MaxRunDurationSeconds int
	RunnerVersion         string
	Idle                  bool
	IdleReason            string
	// VerifyRunner is the resolved verify-pass runner id ("" =
	// reuse execute runner). VerifyRunnerStatus is one of "ok",
	// "demoted_probe_failed", "demoted_build_failed", or "" when
	// no verify runner was requested. Surfaced in the effective
	// config log so operators can confirm the adversarial runner
	// is wired without grepping per-cycle logs.
	VerifyRunner       string
	VerifyRunnerStatus string
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
// attribution keys.
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

// applySettingsSnapshot is the supervisor state read at the start of
// applySettings before any long-running probe/build work.
type applySettingsSnapshot struct {
	cfg  store.AppSettings
	prev *agentWorkerInstance
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

	snap, err := s.loadApplySettingsSnapshot(ctx)
	if err != nil {
		return err
	}

	if idle, reason := decideIdle(snap.cfg); idle {
		return s.handleApplySettingsIdle(phase, snap.cfg, snap.prev, reason)
	}

	version, probeErr := s.probeExecuteRunner(ctx, snap.cfg)
	if probeErr != nil {
		return s.handleApplySettingsProbeFailed(phase, snap.cfg, snap.prev, probeErr)
	}

	if snap.prev != nil && instanceMatchesSettings(snap.prev, snap.cfg, version) {
		return s.handleApplySettingsUnchanged(ctx, phase, snap.cfg, snap.prev, version)
	}

	return s.restartWorkerWithSettings(ctx, phase, snap.cfg, snap.prev, version)
}

func (s *agentWorkerSupervisor) loadApplySettingsSnapshot(ctx context.Context) (*applySettingsSnapshot, error) {
	cfg, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent worker supervisor: read settings: %w", err)
	}

	s.mu.Lock()
	if s.drained {
		s.mu.Unlock()
		return nil, errors.New("agent worker supervisor: already drained")
	}
	prev := s.current
	s.mu.Unlock()

	return &applySettingsSnapshot{cfg: cfg, prev: prev}, nil
}

func baseEffectiveSettings(cfg store.AppSettings) effectiveSettingsLog {
	return effectiveSettingsLog{
		AgentPaused:           cfg.AgentPaused,
		Runner:                cfg.Runner,
		RepoRoot:              cfg.RepoRoot,
		CursorBin:             cfg.CursorBin,
		CursorModel:           cfg.CursorModel,
		MaxRunDurationSeconds: cfg.MaxRunDurationSeconds,
	}
}

func (s *agentWorkerSupervisor) finishApplySettings(phase string, eff effectiveSettingsLog) {
	s.logEffective(phase, eff)
	s.publishSettingsChanged()
}

func (s *agentWorkerSupervisor) clearCurrentInstance(prev *agentWorkerInstance, stopReason string) {
	stopWorkerInstance(prev, stopReason)
	s.mu.Lock()
	if !s.drained {
		s.current = nil
	}
	s.mu.Unlock()
}

func (s *agentWorkerSupervisor) handleApplySettingsIdle(phase string, cfg store.AppSettings, prev *agentWorkerInstance, reason string) error {
	s.clearCurrentInstance(prev, "idle:"+reason)
	eff := baseEffectiveSettings(cfg)
	eff.Idle = true
	eff.IdleReason = reason
	s.finishApplySettings(phase, eff)
	return nil
}

func (s *agentWorkerSupervisor) probeExecuteRunner(ctx context.Context, cfg store.AppSettings) (string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, s.probeBudge)
	defer cancel()
	version, _, probeErr := s.probe(probeCtx, cfg.Runner, cfg.CursorBin, s.probeBudge)
	return version, probeErr
}

func (s *agentWorkerSupervisor) handleApplySettingsProbeFailed(phase string, cfg store.AppSettings, prev *agentWorkerInstance, probeErr error) error {
	s.clearCurrentInstance(prev, "probe_failed")
	slog.Warn("agent worker probe failed; staying idle", "cmd", cmdName,
		"operation", "taskapi.agent_worker.probe_err", "phase", phase,
		"runner", cfg.Runner, "binary", cfg.CursorBin, "err", probeErr)
	eff := baseEffectiveSettings(cfg)
	eff.Idle = true
	eff.IdleReason = "probe_failed"
	s.finishApplySettings(phase, eff)
	return nil
}

func (s *agentWorkerSupervisor) handleApplySettingsUnchanged(ctx context.Context, phase string, cfg store.AppSettings, prev *agentWorkerInstance, version string) error {
	eff := baseEffectiveSettings(cfg)
	eff.RunnerVersion = version
	eff.IdleReason = s.probeSchedulingHint(ctx)
	eff.VerifyRunner = cfg.VerifyRunnerName
	eff.VerifyRunnerStatus = verifyRunnerStatusForInstance(prev, cfg)
	s.finishApplySettings(phase, eff)
	return nil
}

func (s *agentWorkerSupervisor) restartWorkerWithSettings(ctx context.Context, phase string, cfg store.AppSettings, prev *agentWorkerInstance, version string) error {
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
		s.clearCurrentInstance(prev, "build_failed")
		return fmt.Errorf("agent worker build runner %q: %w", cfg.Runner, err)
	}

	next, verifyStatus := s.spawnWorkerInstance(ctx, cfg, r)

	s.mu.Lock()
	if s.drained {
		s.mu.Unlock()
		next.cancelCtx()
		<-next.doneCh
		return errors.New("agent worker supervisor: drained mid-start")
	}
	s.current = next
	s.mu.Unlock()

	stopWorkerInstance(prev, "reload")

	eff := baseEffectiveSettings(cfg)
	eff.RunnerVersion = version
	eff.IdleReason = s.probeSchedulingHint(ctx)
	eff.VerifyRunner = cfg.VerifyRunnerName
	eff.VerifyRunnerStatus = verifyStatus
	s.finishApplySettings(phase, eff)
	return nil
}

func (s *agentWorkerSupervisor) spawnWorkerInstance(ctx context.Context, cfg store.AppSettings, r runner.Runner) (*agentWorkerInstance, string) {
	runTimeout := time.Duration(cfg.MaxRunDurationSeconds) * time.Second
	notifier := newCycleChangeSSEAdapter(s.hub)
	progressNotifier := newRunProgressSSEAdapter(s.hub, agentRunProgressMinInterval)
	verifyRunner, verifyStatus := s.buildVerifyRunner(ctx, cfg)
	reportDir := taskapiconfig.WorkerReportDir()
	if err := ensureWorkerReportDirWritable(reportDir); err != nil {
		// Loud warn, but do NOT block the worker — the cost of a
		// non-writable scratch dir is "verify reports never land",
		// surfaces immediately on the first verify pass, and is
		// fixable from the operator side. Blocking startup would
		// trade a transient mis-config for a dead worker.
		slog.Warn("agent worker report dir not writable; worker will start but verify will fail",
			"cmd", cmdName, "operation", "taskapi.agent_worker.report_dir_not_writable",
			"path", reportDir, "err", err)
	}
	w := worker.NewWorker(s.store, s.queue, r, worker.Options{
		RunTimeout:       runTimeout,
		WorkingDir:       cfg.RepoRoot,
		ReportDir:        reportDir,
		Notifier:         notifier,
		ProgressNotifier: progressNotifier,
		Metrics:          s.metrics,
		VerifyRunner:     verifyRunner,
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

	return &agentWorkerInstance{
		worker: w, cancelCtx: cancelWorker, doneCh: done,
		runTimeout: runTimeout, settings: cfg, runner: r,
		verifyRunner: verifyRunner,
	}, verifyStatus
}

// buildVerifyRunner returns the verify-pass runner the worker should
// use, plus a status label for the effective-config log. Returns
// (nil, "") when the operator did not configure VerifyRunnerName, in
// which case the worker reuses the execute runner (V1 behaviour).
//
// Build / probe failures DO NOT block worker startup. The cost of an
// opt-in feature failure is its own absence — verify silently demotes
// to "reuse execute runner" with a loud warn, the worker still picks
// up tasks, and the operator sees "demoted_*" in the effective config
// log so they can fix the misconfiguration without losing throughput.
func (s *agentWorkerSupervisor) buildVerifyRunner(ctx context.Context, cfg store.AppSettings) (runner.Runner, string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.buildVerifyRunner",
		"verify_runner", cfg.VerifyRunnerName)
	if cfg.VerifyRunnerName == "" {
		return nil, ""
	}
	if cfg.VerifyRunnerName == cfg.Runner {
		// Operator picked the same id as execute; treat as "reuse"
		// without paying for a second build/probe. Distinct status
		// so logs explain "verify_runner=cursor" + "status=reuse".
		return nil, "reuse_execute_runner"
	}
	probeCtx, cancel := context.WithTimeout(ctx, s.probeBudge)
	version, _, probeErr := s.probe(probeCtx, cfg.VerifyRunnerName, cfg.CursorBin, s.probeBudge)
	cancel()
	if probeErr != nil {
		slog.Warn("verify_runner_probe_failed; demoting to execute runner",
			"cmd", cmdName, "operation", "taskapi.agent_worker.verify_runner_probe_err",
			"runner", cfg.VerifyRunnerName, "binary", cfg.CursorBin, "err", probeErr)
		return nil, "demoted_probe_failed"
	}
	r, err := registry.Build(cfg.VerifyRunnerName, registry.BuildOptions{
		BinaryPath:  cfg.CursorBin,
		Version:     version,
		CursorModel: cfg.VerifyRunnerModel,
	})
	if err != nil {
		slog.Warn("verify_runner_build_failed; demoting to execute runner",
			"cmd", cmdName, "operation", "taskapi.agent_worker.verify_runner_build_err",
			"runner", cfg.VerifyRunnerName, "err", err)
		return nil, "demoted_build_failed"
	}
	return r, "ok"
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
	slog.Info("agent worker startup finalize ok", "cmd", cmdName,
		"operation", "taskapi.agent_worker.finalize_ok",
		"phases_finalized", res.PhasesFailed)
	return nil
}

func (s *agentWorkerSupervisor) logEffective(phase string, eff effectiveSettingsLog) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.agentWorkerSupervisor.logEffective",
		"phase", phase)
	slog.Info("agent worker effective config", "cmd", cmdName, "operation", "taskapi.agent_worker",
		"phase", phase,
		"paused", eff.AgentPaused,
		"idle", eff.Idle, "idle_reason", eff.IdleReason,
		"runner", eff.Runner, "runner_version", eff.RunnerVersion,
		"repo_root", eff.RepoRoot, "cursor_bin", eff.CursorBin,
		"cursor_model", eff.CursorModel,
		"max_run_duration_sec", eff.MaxRunDurationSeconds,
		"verify_runner", eff.VerifyRunner, "verify_runner_status", eff.VerifyRunnerStatus)
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

// SchedulingIdleHintReason re-exports the policy constant for cmd tests.
const SchedulingIdleHintReason = policy.SchedulingIdleHintReason

func decideSchedulingIdleHint(queueEmpty bool, scheduledCount int64) string {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.decideSchedulingIdleHint",
		"queue_empty", queueEmpty, "scheduled_count", scheduledCount)
	return policy.DecideSchedulingIdleHint(queueEmpty, scheduledCount)
}

func decideIdle(cfg store.AppSettings) (bool, string) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.decideIdle",
		"paused", cfg.AgentPaused, "repo_root", cfg.RepoRoot)
	idle, reason := policy.DecideIdle(cfg, assertWorkingDirExists)
	if reason == "repo_root_invalid" {
		slog.Warn("agent worker repo root not usable; staying idle",
			"cmd", cmdName, "operation", "taskapi.agent_worker.repo_root_err",
			"path", cfg.RepoRoot, "err", assertWorkingDirExists(cfg.RepoRoot))
	}
	return idle, reason
}

func instanceSnapshot(inst *agentWorkerInstance, version string) *policy.InstanceSnapshot {
	if inst == nil {
		return nil
	}
	snap := &policy.InstanceSnapshot{Settings: inst.settings}
	if inst.runner != nil {
		if version != "" {
			snap.RunnerVersion = version
		} else {
			snap.RunnerVersion = inst.runner.Version()
		}
	}
	snap.HasVerifyRunner = inst.verifyRunner != nil
	return snap
}

func instanceMatchesSettings(inst *agentWorkerInstance, cfg store.AppSettings, version string) bool {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.instanceMatchesSettings")
	return policy.InstanceMatchesSettings(instanceSnapshot(inst, version), cfg, version)
}

func verifyRunnerStatusForInstance(prev *agentWorkerInstance, cfg store.AppSettings) string {
	hasVerify := prev != nil && prev.verifyRunner != nil
	return policy.VerifyRunnerStatus(hasVerify, cfg)
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
// reconcile loop is always on; the worker is gated on AppSettings.AgentPaused
// and dependencies (repo root, runner probe). The returned cancel func stops
// pickup wake and tears down the reconcile goroutine; the supervisor owns
// the worker lifecycle and exposes Drain for shutdown.
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
	go agents.RunReconcileLoop(reconcileCtx, taskStore, agentQueue, iv, nil)

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

// ensureWorkerReportDirWritable creates the worker-managed scratch
// directory if it does not exist and confirms the worker process can
// write into it by touching a sentinel file. Distinct from
// assertWorkingDirExists because RepoRoot is operator-supplied and
// MUST exist (idle if missing), while the report dir is a worker
// internal detail that we create on demand. A failure here is a warn,
// not a fatal: see the call site in applySettings for the rationale.
func ensureWorkerReportDirWritable(dir string) error {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.ensureWorkerReportDirWritable",
		"dir", dir)
	if dir == "" {
		return errors.New("report dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".t2a-worker-probe-*")
	if err != nil {
		return fmt.Errorf("write probe in %q: %w", dir, err)
	}
	probePath := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probePath)
	return nil
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
// the SPA cache invalidation hook are pinned by docs/api.md and
// docs/data-model.md.
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

type runProgressSSEAdapter struct {
	hub         *handler.SSEHub
	minInterval time.Duration

	mu       sync.Mutex
	lastSent map[string]time.Time
}

func newRunProgressSSEAdapter(hub *handler.SSEHub, minInterval time.Duration) *runProgressSSEAdapter {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.newRunProgressSSEAdapter")
	return &runProgressSSEAdapter{
		hub:         hub,
		minInterval: minInterval,
		lastSent:    make(map[string]time.Time),
	}
}

func (a *runProgressSSEAdapter) PublishRunProgress(taskID, cycleID string, phaseSeq int64, ev runner.ProgressEvent) {
	slog.Debug("trace", "cmd", cmdName, "operation", "taskapi.runProgressSSEAdapter.PublishRunProgress",
		"task_id", taskID, "cycle_id", cycleID, "phase_seq", phaseSeq,
		"kind", ev.Kind, "subtype", ev.Subtype)
	if a == nil || a.hub == nil || taskID == "" || cycleID == "" || phaseSeq <= 0 || ev.Kind == "" {
		return
	}
	if a.shouldDrop(taskID, cycleID, phaseSeq) {
		return
	}
	a.hub.Publish(handler.TaskChangeEvent{
		Type:     handler.AgentRunProgress,
		ID:       taskID,
		CycleID:  cycleID,
		PhaseSeq: phaseSeq,
		Progress: &handler.AgentRunProgressPayload{
			Kind:    ev.Kind,
			Subtype: ev.Subtype,
			Message: ev.Message,
			Tool:    ev.Tool,
		},
	})
}

func (a *runProgressSSEAdapter) shouldDrop(taskID, cycleID string, phaseSeq int64) bool {
	if a.minInterval <= 0 {
		return false
	}
	key := fmt.Sprintf("%s:%s:%d", taskID, cycleID, phaseSeq)
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	last, ok := a.lastSent[key]
	if ok && now.Sub(last) < a.minInterval {
		return true
	}
	a.lastSent[key] = now
	if len(a.lastSent) > agentRunProgressThrottleEntries {
		for old := range a.lastSent {
			if old != key {
				delete(a.lastSent, old)
				break
			}
		}
	}
	return false
}
