package verify

import (
	"context"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/git"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) checkIntegrity(ctx context.Context, cycleID string, pre git.IntegritySnapshot, preErr error) (bool, string) {
	if s.git == nil {
		return git.CheckVerifyIntegrity(ctx, nil, s.workingDir, cycleID, pre, preErr)
	}
	return git.CheckVerifyIntegrity(ctx, s.git.Repo(), s.workingDir, cycleID, pre, preErr)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) captureIntegritySnapshot(ctx context.Context) (git.IntegritySnapshot, error) {
	repo := s.git.Repo()
	return git.CaptureIntegritySnapshot(ctx, repo, s.workingDir)
}
