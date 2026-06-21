package agentworker

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/taskapi/agentworker/policy"
	"github.com/AlexsanderHamir/Hamix/internal/taskapiconfig"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/worker"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

type instance struct {
	worker       *worker.Worker
	cancelCtx    context.CancelFunc
	doneCh       chan struct{}
	runTimeout   time.Duration
	settings     store.AppSettings
	runner       runner.Runner
	verifyRunner runner.Runner
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func instanceSnapshot(inst *instance, version string) *policy.InstanceSnapshot {
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

func instanceMatchesSettings(inst *instance, cfg store.AppSettings, version string) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.instanceMatchesSettings")
	return policy.InstanceMatchesSettings(instanceSnapshot(inst, version), cfg, version)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func verifyRunnerStatusForInstance(prev *instance, cfg store.AppSettings) string {
	hasVerify := prev != nil && prev.verifyRunner != nil
	return policy.VerifyRunnerStatus(hasVerify, cfg)
}

func stopWorkerInstance(inst *instance, reason string) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.stopWorkerInstance",
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
		slog.Info("agent worker instance stopped", "cmd", logCmd,
			"operation", "taskapi.agent_worker.stop", "reason", reason)
	case <-time.After(deadline):
		slog.Warn("agent worker instance drain timeout", "cmd", logCmd,
			"operation", "taskapi.agent_worker.stop_timeout",
			"reason", reason, "deadline", deadline.String())
	}
}

func (s *Supervisor) spawnWorkerInstance(ctx context.Context, cfg store.AppSettings, r runner.Runner) (*instance, string) {
	runTimeout := time.Duration(cfg.MaxRunDurationSeconds) * time.Second
	streamIdleStuck := time.Duration(cfg.StreamIdleStuckSeconds) * time.Second
	notifier := newCycleChangeSSEAdapter(s.publisher)
	progressNotifier := newRunProgressSSEAdapter(s.publisher, agentRunProgressMinInterval)
	verifyRunner, verifyStatus := s.buildVerifyRunner(ctx, cfg)
	reportDir := taskapiconfig.WorkerReportDir()
	if err := ensureWorkerReportDirWritable(reportDir); err != nil {
		slog.Warn("agent worker report dir not writable; worker will start but verify will fail",
			"cmd", logCmd, "operation", "taskapi.agent_worker.report_dir_not_writable",
			"path", reportDir, "err", err)
	}
	w := worker.NewWorker(s.store, s.queue, r, worker.Options{
		RunTimeout:       runTimeout,
		StreamIdleStuck:  streamIdleStuck,
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
			slog.Error("agent worker exited with error", "cmd", logCmd,
				"operation", "taskapi.agent_worker.exit_err", "err", err)
		}
	}()

	return &instance{
		worker: w, cancelCtx: cancelWorker, doneCh: done,
		runTimeout: runTimeout, settings: cfg, runner: r,
		verifyRunner: verifyRunner,
	}, verifyStatus
}
