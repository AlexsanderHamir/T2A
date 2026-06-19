package git

import (
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// Service orchestrates harness git operations with explicit dependencies.
type Service struct {
	store     *store.Store
	gitRepo   GitRepo
	reportDir string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NewService constructs a git Service. gitRepo defaults to ExecRepo when nil.
func NewService(st *store.Store, gitRepo GitRepo, reportDir string) *Service {
	if gitRepo == nil {
		gitRepo = NewExecRepo()
	}
	return &Service{store: st, gitRepo: gitRepo, reportDir: reportDir}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) repo() GitRepo {
	if s.gitRepo == nil {
		return DefaultRepo()
	}
	return s.gitRepo
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// Repo returns the configured GitRepo (never nil).
func (s *Service) Repo() GitRepo {
	return s.repo()
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ReportDir returns the worker report directory for criteria-report reads.
func (s *Service) ReportDir() string {
	return s.reportDir
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// SetReportDir updates the report directory (used when opts change after construction).
func (s *Service) SetReportDir(dir string) {
	s.reportDir = dir
}
