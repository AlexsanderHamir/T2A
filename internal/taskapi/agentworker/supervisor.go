package agentworker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/taskapi"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const logCmd = "taskapi"

const (
	shutdownGraceAfterRunTimeout   = 10 * time.Second
	drainNoLimitTimeout            = 5 * time.Minute
	agentWorkerStartupSweepTimeout = 30 * time.Second
)

// Supervisor owns the lifecycle of the in-process agent worker.
// Construct via New; drive with Start, Reload, CancelCurrentRun, Drain.
// Methods are safe for concurrent use; applyMu serialises Start/Reload
// end-to-end so concurrent settings patches cannot leak worker goroutines.
type Supervisor struct {
	parentCtx  context.Context
	store      *store.Store
	queue      *agents.MemoryQueue
	hub        *handler.SSEHub
	metrics    worker.RunMetrics
	probe      func(ctx context.Context, id, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error)
	probeBudge time.Duration

	applyMu sync.Mutex

	mu      sync.Mutex
	current *instance
	drained bool
}

// New wires the supervisor with its dependencies. The supervisor does
// not start the worker; the caller invokes Start once after the rest
// of taskapi assembly finishes.
func New(ctx context.Context, st *store.Store, q *agents.MemoryQueue, hub *handler.SSEHub) *Supervisor {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.newAgentWorkerSupervisor")
	return &Supervisor{
		parentCtx:  ctx,
		store:      st,
		queue:      q,
		hub:        hub,
		metrics:    taskapi.RegisterAgentWorkerMetrics(),
		probe:      registry.Probe,
		probeBudge: 5 * time.Second,
	}
}

// Start performs the first boot of the worker by delegating to applySettings.
func (s *Supervisor) Start(ctx context.Context) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.Start")
	s.logPreFeatureCycleCount(ctx)
	return s.applySettings(ctx, "boot")
}

func (s *Supervisor) logPreFeatureCycleCount(ctx context.Context) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.logPreFeatureCycleCount")
	countCtx, cancel := context.WithTimeout(ctx, agentWorkerStartupSweepTimeout)
	defer cancel()
	counts, err := s.store.CountPreFeatureCycles(countCtx)
	if err != nil {
		slog.Warn("agent worker pre-feature cycle count skipped",
			"cmd", logCmd,
			"operation", "taskapi.agent_worker.pre_feature_count_err",
			"err", err)
		return
	}
	slog.Info("agent worker pre-feature cycles",
		"cmd", logCmd,
		"operation", "taskapi.agentWorkerSupervisor.startup.preFeatureCycleCount",
		"terminal_cycles_total", counts.Total,
		"missing_cursor_model_effective_key", counts.MissingKey,
		"empty_cursor_model_effective_value", counts.EmptyValue)
}

// Reload re-reads AppSettings and respawns the worker if anything material changed.
func (s *Supervisor) Reload(ctx context.Context) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.Reload")
	return s.applySettings(ctx, "reload")
}

// ProbeRunner exposes the runner registry probe to the HTTP handler.
func (s *Supervisor) ProbeRunner(ctx context.Context, runnerID, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.ProbeRunner",
		"runner", runnerID, "binary", binaryPath)
	if timeout <= 0 {
		timeout = s.probeBudge
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.probe(probeCtx, runnerID, binaryPath, timeout)
}

// CancelCurrentRun cancels the in-flight runner.Run, if any.
func (s *Supervisor) CancelCurrentRun() bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.CancelCurrentRun")
	s.mu.Lock()
	inst := s.current
	s.mu.Unlock()
	if inst == nil || inst.worker == nil {
		return false
	}
	return inst.worker.CancelCurrentRun()
}

// Drain cancels the worker context and waits for Worker.Run to return.
func (s *Supervisor) Drain() {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.agentWorkerSupervisor.Drain")
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
