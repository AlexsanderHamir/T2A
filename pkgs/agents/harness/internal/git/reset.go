package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ResetForFreshRetry resets the working tree to the parent cycle anchor before fresh retry.
func (s *Service) ResetForFreshRetry(ctx context.Context, workingDir, parentCycleID string) (FreshRetryResetOutcome, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.ResetForFreshRetry",
		"parent_cycle_id", parentCycleID)
	workdir := strings.TrimSpace(workingDir)
	if workdir == "" {
		return FreshRetryResetOutcome{Skipped: true}, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, gitSnapshotProbeTimeout)
	defer cancel()
	if _, err := s.repo().Run(probeCtx, workdir, "rev-parse", "HEAD"); err != nil {
		if IsNotAGitRepoErr(err) {
			return FreshRetryResetOutcome{Skipped: true}, nil
		}
		return FreshRetryResetOutcome{}, fmt.Errorf("%s: %w", RetryGitResetFailed, err)
	}
	anchor, err := s.resolveFreshRetryAnchor(ctx, workdir, parentCycleID)
	if err != nil {
		return FreshRetryResetOutcome{}, err
	}
	if strings.TrimSpace(anchor) == "" {
		return FreshRetryResetOutcome{}, errors.New(RetryResetAnchorMissing)
	}
	resetCtx, resetCancel := context.WithTimeout(ctx, gitSnapshotProbeTimeout)
	defer resetCancel()
	if err := resetHardClean(resetCtx, s.repo(), workdir, anchor); err != nil {
		return FreshRetryResetOutcome{}, fmt.Errorf("%s: %w", RetryGitResetFailed, err)
	}
	slog.Info("agent harness fresh retry git reset", "cmd", logCmd,
		"operation", "agent.harness.fresh_retry.git_reset",
		"parent_cycle_id", parentCycleID, "anchor", anchor, "workdir", workdir)
	return FreshRetryResetOutcome{}, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) resolveFreshRetryAnchor(ctx context.Context, workdir, parentCycleID string) (string, error) {
	parentCycleID = strings.TrimSpace(parentCycleID)
	if parentCycleID == "" {
		return "", errors.New(RetryResetAnchorMissing)
	}
	phases, err := s.store.ListPhasesForCycle(ctx, parentCycleID)
	if err != nil {
		return "", err
	}
	var firstExecute *domain.TaskCyclePhase
	for i := range phases {
		p := &phases[i]
		if p.Phase != domain.PhaseExecute {
			continue
		}
		if firstExecute == nil || p.PhaseSeq < firstExecute.PhaseSeq {
			firstExecute = p
		}
	}
	if firstExecute != nil {
		if anchor := CycleBaseFromPhaseDetails(firstExecute.DetailsJSON); anchor != "" {
			return anchor, nil
		}
	}
	commits, err := s.store.ListCommitsForCycle(ctx, parentCycleID)
	if err != nil {
		return "", err
	}
	if len(commits) == 0 {
		return "", errors.New(RetryResetAnchorMissing)
	}
	firstSHA := strings.TrimSpace(commits[0].SHA)
	if firstSHA == "" {
		return "", errors.New(RetryResetAnchorMissing)
	}
	if workdir == "" {
		worktree := strings.TrimSpace(commits[0].Worktree)
		if worktree == "" {
			workdir = strings.TrimSpace(commits[0].Repo)
		} else {
			workdir = worktree
		}
	}
	parent, err := s.repo().Run(ctx, workdir, "rev-parse", firstSHA+"^")
	if err != nil {
		return "", errors.New(RetryResetAnchorMissing)
	}
	return strings.TrimSpace(parent), nil
}

// ResolveFreshRetryAnchor resolves the git anchor for a fresh operator retry.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) ResolveFreshRetryAnchor(ctx context.Context, workingDir, parentCycleID string) (string, error) {
	return s.resolveFreshRetryAnchor(ctx, workingDir, parentCycleID)
}

// ResetHardClean runs git reset --hard and git clean -fd (test seam).
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ResetHardClean(ctx context.Context, repo GitRepo, workdir, anchor string) error {
	return resetHardClean(ctx, repo, workdir, anchor)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func resetHardClean(ctx context.Context, repo GitRepo, workdir, anchor string) error {
	if _, err := repo.Run(ctx, workdir, "reset", "--hard", anchor); err != nil {
		return err
	}
	if _, err := repo.Run(ctx, workdir, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}
