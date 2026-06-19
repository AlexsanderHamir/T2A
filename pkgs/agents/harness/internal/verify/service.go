package verify

import (
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// Service runs the verify pipeline stages against explicit dependencies.
type Service struct {
	store        *store.Store
	runner       runner.Runner
	verifyRunner runner.Runner
	reportDir    string
	workingDir   string
	git          *git.Service
	clock        func() time.Time
	hooks        Hooks
}

// Deps bundles Service construction inputs from harness root.
type Deps struct {
	Store        *store.Store
	Runner       runner.Runner
	VerifyRunner runner.Runner
	ReportDir    string
	WorkingDir   string
	Git          *git.Service
	Clock        func() time.Time
	Hooks        Hooks
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NewService constructs a verify Service. VerifyRunner falls back to Runner when nil.
func NewService(deps Deps) *Service {
	verifyRunner := deps.Runner
	if deps.VerifyRunner != nil {
		verifyRunner = deps.VerifyRunner
	}
	clock := deps.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		store:        deps.Store,
		runner:       deps.Runner,
		verifyRunner: verifyRunner,
		reportDir:    deps.ReportDir,
		workingDir:   deps.WorkingDir,
		git:          deps.Git,
		clock:        clock,
		hooks:        deps.Hooks,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) SetReportDir(dir string) {
	s.reportDir = dir
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) SetWorkingDir(dir string) {
	s.workingDir = dir
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) SetVerifyRunner(r runner.Runner) {
	if r != nil {
		s.verifyRunner = r
	} else {
		s.verifyRunner = s.runner
	}
}

func (s *Service) SetStreamIdleStuck(d time.Duration) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.SetStreamIdleStuck",
		"stuck_ns", int64(d))
	s.hooks.StreamIdleStuck = d
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) publish(taskID, cycleID string) {
	if s.hooks.Publish != nil {
		s.hooks.Publish(taskID, cycleID)
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) recordVerdict(kind domain.VerifierKind, passed bool) {
	if s.hooks.RecordVerdict != nil {
		s.hooks.RecordVerdict(kind, passed)
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) observeDuration(d time.Duration) {
	if s.hooks.ObserveDuration != nil {
		if d < 0 {
			d = 0
		}
		s.hooks.ObserveDuration(d)
	}
}
