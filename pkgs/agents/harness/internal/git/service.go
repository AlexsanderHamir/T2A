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

// NewService constructs a git Service. gitRepo defaults to ExecRepo when nil.
func NewService(st *store.Store, gitRepo GitRepo, reportDir string) *Service {
	if gitRepo == nil {
		gitRepo = NewExecRepo()
	}
	return &Service{store: st, gitRepo: gitRepo, reportDir: reportDir}
}

func (s *Service) repo() GitRepo {
	if s.gitRepo == nil {
		return DefaultRepo()
	}
	return s.gitRepo
}

// Repo returns the configured GitRepo (never nil).
func (s *Service) Repo() GitRepo {
	return s.repo()
}

// ReportDir returns the worker report directory for criteria-report reads.
func (s *Service) ReportDir() string {
	return s.reportDir
}

// SetReportDir updates the report directory (used when opts change after construction).
func (s *Service) SetReportDir(dir string) {
	s.reportDir = dir
}
