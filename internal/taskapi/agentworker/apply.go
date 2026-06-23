package agentworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/internal/taskapi/agentworker/policy"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry"
	_ "github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry/all"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/realtime"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type applySettingsSnapshot struct {
	cfg  store.AppSettings
	prev *instance
}

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
	VerifyRunner          string
	VerifyRunnerStatus    string
}

func (s *Supervisor) applySettings(ctx context.Context, phase string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.applySettings",
		"phase", phase)
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	snap, err := s.loadApplySettingsSnapshot(ctx)
	if err != nil {
		return err
	}

	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.decideIdle",
		"paused", snap.cfg.AgentPaused)
	idle, reason := policy.DecideIdle(ctx, snap.cfg, s.gitRegistrationChecker)
	if idle {
		if reason == "all_worktrees_invalid" {
			slog.Warn("agent worker git worktrees not usable; staying idle",
				"cmd", logCmd, "operation", "taskapi.agent_worker.worktrees_invalid")
		}
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) loadApplySettingsSnapshot(ctx context.Context) (*applySettingsSnapshot, error) {
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) finishApplySettings(phase string, eff effectiveSettingsLog) {
	s.logEffective(phase, eff)
	s.publishSettingsChanged()
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) clearCurrentInstance(prev *instance, stopReason string) {
	stopWorkerInstance(prev, stopReason)
	s.mu.Lock()
	if !s.drained {
		s.current = nil
	}
	s.mu.Unlock()
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) handleApplySettingsIdle(phase string, cfg store.AppSettings, prev *instance, reason string) error {
	s.clearCurrentInstance(prev, "idle:"+reason)
	eff := baseEffectiveSettings(cfg)
	eff.Idle = true
	eff.IdleReason = reason
	s.finishApplySettings(phase, eff)
	return nil
}

func (s *Supervisor) handleApplySettingsProbeFailed(phase string, cfg store.AppSettings, prev *instance, probeErr error) error {
	s.clearCurrentInstance(prev, "probe_failed")
	slog.Warn("agent worker probe failed; staying idle", "cmd", logCmd,
		"operation", "taskapi.agent_worker.probe_err", "phase", phase,
		"runner", cfg.Runner, "binary", cfg.CursorBin, "err", probeErr)
	eff := baseEffectiveSettings(cfg)
	eff.Idle = true
	eff.IdleReason = "probe_failed"
	s.finishApplySettings(phase, eff)
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) handleApplySettingsUnchanged(ctx context.Context, phase string, cfg store.AppSettings, prev *instance, version string) error {
	eff := baseEffectiveSettings(cfg)
	eff.RunnerVersion = version
	eff.IdleReason = s.probeSchedulingHint(ctx)
	eff.VerifyRunner = cfg.VerifyRunnerName
	eff.VerifyRunnerStatus = verifyRunnerStatusForInstance(prev, cfg)
	s.finishApplySettings(phase, eff)
	return nil
}

func (s *Supervisor) restartWorkerWithSettings(ctx context.Context, phase string, cfg store.AppSettings, prev *instance, version string) error {
	if err := s.runStartupSweep(ctx); err != nil {
		slog.Warn("agent worker startup sweep failed (continuing)",
			"cmd", logCmd, "operation", "taskapi.agent_worker.sweep_err",
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

func (s *Supervisor) logEffective(phase string, eff effectiveSettingsLog) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.logEffective",
		"phase", phase)
	slog.Info("agent worker effective config", "cmd", logCmd, "operation", "taskapi.agent_worker",
		"phase", phase,
		"paused", eff.AgentPaused,
		"idle", eff.Idle, "idle_reason", eff.IdleReason,
		"runner", eff.Runner, "runner_version", eff.RunnerVersion,
		"repo_root", eff.RepoRoot, "cursor_bin", eff.CursorBin,
		"cursor_model", eff.CursorModel,
		"max_run_duration_sec", eff.MaxRunDurationSeconds,
		"verify_runner", eff.VerifyRunner, "verify_runner_status", eff.VerifyRunnerStatus)
}

func (s *Supervisor) publishSettingsChanged() {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.publishSettingsChanged")
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(realtime.Event{Type: realtime.SettingsChanged})
}

//funclogmeasure:skip category=delegate-already-logs reason="Thin wrapper; store.AgentWorkerGitIdle emits the persistence trace."
func (s *Supervisor) gitRegistrationChecker(ctx context.Context) (idle bool, reason string, err error) {
	return s.store.AgentWorkerGitIdle(ctx)
}
