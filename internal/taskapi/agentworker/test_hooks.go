package agentworker

import (
	"context"
	"time"
	"unsafe"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// HasRunningInstance reports whether a worker instance is currently active.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) HasRunningInstance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current != nil
}

// RunningInstanceIdentity returns an opaque identity for the current
// instance, or 0 when idle. Tests use this to detect respawn without
// exposing internal instance types.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) RunningInstanceIdentity() uintptr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(s.current))
}

// RunningInstanceRunnerVersion returns the execute runner version when active.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) RunningInstanceRunnerVersion() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || s.current.runner == nil {
		return "", false
	}
	return s.current.runner.Version(), true
}

// SetProbeForTest replaces the registry probe (cmd tests only).
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) SetProbeForTest(fn func(ctx context.Context, id, binaryPath string, timeout time.Duration) (string, string, error)) {
	s.probe = fn
}

// SetProbeBudgetForTest overrides the probe timeout budget (cmd tests only).
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Supervisor) SetProbeBudgetForTest(d time.Duration) {
	s.probeBudge = d
}

// BuildVerifyRunnerForTest exposes buildVerifyRunner for cmd tests.
func (s *Supervisor) BuildVerifyRunnerForTest(ctx context.Context, cfg store.AppSettings) (runner.Runner, string) {
	return s.buildVerifyRunner(ctx, cfg)
}

// ProbeSchedulingHintForTest exposes probeSchedulingHint for cmd tests.
func (s *Supervisor) ProbeSchedulingHintForTest(ctx context.Context) string {
	return s.probeSchedulingHint(ctx)
}
