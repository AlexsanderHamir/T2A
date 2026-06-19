package resume

import (
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const logCmd = "taskapi"

// Options configures resume checkpoint and continuation loading.
type Options struct {
	WorkingDir string
	GitRepo    git.GitRepo
}

// Service loads checkpoints and continuation bundles from the store and phase ledger.
type Service struct {
	store *store.Store
	opts  Options
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NewService constructs a resume Service. GitRepo defaults to ExecRepo when nil.
func NewService(st *store.Store, opts Options) *Service {
	if opts.GitRepo == nil {
		opts.GitRepo = git.NewExecRepo()
	}
	return &Service{store: st, opts: opts}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) gitRepo() git.GitRepo {
	if s.opts.GitRepo == nil {
		return git.DefaultRepo()
	}
	return s.opts.GitRepo
}
