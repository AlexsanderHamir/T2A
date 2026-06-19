package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const (
	retryResetAnchorMissing = "retry_reset_anchor_missing"
	retryGitResetFailed     = "retry_git_reset_failed"
)

type freshRetryResetOutcome struct {
	skipped bool
}

// gitResetForFreshRetry resets the working tree to the parent cycle anchor
// before a fresh operator retry. Non-git workdirs skip reset; missing
// anchors and git command failures fail loud with stable reason strings.
func (h *Harness) gitResetForFreshRetry(ctx context.Context, parentCycleID string) (freshRetryResetOutcome, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.gitResetForFreshRetry",
		"parent_cycle_id", parentCycleID)
	workdir := strings.TrimSpace(h.opts.WorkingDir)
	if workdir == "" {
		return freshRetryResetOutcome{skipped: true}, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, gitSnapshotProbeTimeout)
	defer cancel()
	if _, err := runGit(probeCtx, workdir, "rev-parse", "HEAD"); err != nil {
		if isNotAGitRepoErr(err) {
			return freshRetryResetOutcome{skipped: true}, nil
		}
		return freshRetryResetOutcome{}, fmt.Errorf("%s: %w", retryGitResetFailed, err)
	}
	anchor, err := h.resolveFreshRetryAnchor(ctx, parentCycleID)
	if err != nil {
		return freshRetryResetOutcome{}, err
	}
	if strings.TrimSpace(anchor) == "" {
		return freshRetryResetOutcome{}, errors.New(retryResetAnchorMissing)
	}
	resetCtx, resetCancel := context.WithTimeout(ctx, gitSnapshotProbeTimeout)
	defer resetCancel()
	if err := gitResetHardClean(resetCtx, workdir, anchor); err != nil {
		return freshRetryResetOutcome{}, fmt.Errorf("%s: %w", retryGitResetFailed, err)
	}
	slog.Info("agent harness fresh retry git reset", "cmd", harnessLogCmd,
		"operation", "agent.harness.fresh_retry.git_reset",
		"parent_cycle_id", parentCycleID, "anchor", anchor, "workdir", workdir)
	return freshRetryResetOutcome{}, nil
}

func (h *Harness) resolveFreshRetryAnchor(ctx context.Context, parentCycleID string) (string, error) {
	parentCycleID = strings.TrimSpace(parentCycleID)
	if parentCycleID == "" {
		return "", errors.New(retryResetAnchorMissing)
	}
	phases, err := h.store.ListPhasesForCycle(ctx, parentCycleID)
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
		if anchor := gitCycleBaseFromPhaseDetails(firstExecute.DetailsJSON); anchor != "" {
			return anchor, nil
		}
	}
	commits, err := h.store.ListCommitsForCycle(ctx, parentCycleID)
	if err != nil {
		return "", err
	}
	if len(commits) == 0 {
		return "", errors.New(retryResetAnchorMissing)
	}
	firstSHA := strings.TrimSpace(commits[0].SHA)
	if firstSHA == "" {
		return "", errors.New(retryResetAnchorMissing)
	}
	workdir := strings.TrimSpace(h.opts.WorkingDir)
	if workdir == "" {
		worktree := strings.TrimSpace(commits[0].Worktree)
		if worktree == "" {
			workdir = strings.TrimSpace(commits[0].Repo)
		} else {
			workdir = worktree
		}
	}
	parent, err := runGit(ctx, workdir, "rev-parse", firstSHA+"^")
	if err != nil {
		return "", errors.New(retryResetAnchorMissing)
	}
	return strings.TrimSpace(parent), nil
}

func gitResetHardClean(ctx context.Context, workdir, anchor string) error {
	if _, err := runGit(ctx, workdir, "reset", "--hard", anchor); err != nil {
		return err
	}
	if _, err := runGit(ctx, workdir, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}
