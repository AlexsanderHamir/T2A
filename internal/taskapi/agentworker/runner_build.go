package agentworker

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi/agentworker/policy"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func (s *Supervisor) buildVerifyRunner(ctx context.Context, cfg store.AppSettings) (runner.Runner, string) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.buildVerifyRunner",
		"verify_runner", cfg.VerifyRunnerName)
	if cfg.VerifyRunnerName == "" {
		return nil, ""
	}
	if cfg.VerifyRunnerName == cfg.Runner {
		return nil, "reuse_execute_runner"
	}
	probeCtx, cancel := context.WithTimeout(ctx, s.probeBudge)
	version, _, probeErr := s.probe(probeCtx, cfg.VerifyRunnerName, cfg.CursorBin, s.probeBudge)
	cancel()
	if probeErr != nil {
		slog.Warn("verify_runner_probe_failed; demoting to execute runner",
			"cmd", logCmd, "operation", "taskapi.agent_worker.verify_runner_probe_err",
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
			"cmd", logCmd, "operation", "taskapi.agent_worker.verify_runner_build_err",
			"runner", cfg.VerifyRunnerName, "err", err)
		return nil, "demoted_build_failed"
	}
	return r, "ok"
}

func (s *Supervisor) probeExecuteRunner(ctx context.Context, cfg store.AppSettings) (string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, s.probeBudge)
	defer cancel()
	version, _, probeErr := s.probe(probeCtx, cfg.Runner, cfg.CursorBin, s.probeBudge)
	return version, probeErr
}

func (s *Supervisor) runStartupSweep(ctx context.Context) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.runStartupSweep")
	sweepCtx, cancel := context.WithTimeout(ctx, agentWorkerStartupSweepTimeout)
	defer cancel()
	res, err := worker.SweepOrphanRunningCycles(sweepCtx, s.store)
	if err != nil {
		return err
	}
	slog.Info("agent worker startup finalize ok", "cmd", logCmd,
		"operation", "taskapi.agent_worker.finalize_ok",
		"phases_finalized", res.PhasesFailed)
	return nil
}

func (s *Supervisor) probeSchedulingHint(ctx context.Context) string {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.probeSchedulingHint")
	if s.store == nil {
		return ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	candidates, err := s.store.ListReadyTaskQueueCandidates(probeCtx, 1, nil)
	if err != nil {
		slog.Debug("scheduling hint: queue probe failed",
			"cmd", logCmd, "operation", "taskapi.agent_worker.scheduling_hint_queue_err",
			"err", err)
		return ""
	}
	queueEmpty := len(candidates) == 0
	if !queueEmpty {
		return ""
	}
	stats, err := s.store.TaskStats(probeCtx)
	if err != nil {
		slog.Debug("scheduling hint: stats probe failed",
			"cmd", logCmd, "operation", "taskapi.agent_worker.scheduling_hint_stats_err",
			"err", err)
		return ""
	}
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.decideSchedulingIdleHint",
		"queue_empty", queueEmpty, "scheduled_count", stats.Scheduled)
	return policy.DecideSchedulingIdleHint(queueEmpty, stats.Scheduled)
}
