package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const maxCriteriaReportFileBytes = 256 * 1024

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) parseCommitReports(reportDir, cycleID string) ([]commitReport, error) {
	path := reports.CriteriaReportPath(reportDir, cycleID)
	var rep struct {
		Criteria []reports.CriteriaEntry `json:"criteria"`
		Commits  []commitReport          `json:"commits"`
	}
	if err := readCriteriaReportJSON(path, &rep); err != nil {
		if errors.Is(err, reports.ErrCriteriaReportMissing) {
			return nil, nil
		}
		return nil, err
	}
	return rep.Commits, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func readCriteriaReportJSON(path string, dest any) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reports.ErrCriteriaReportMissing
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: symlink not permitted", reports.ErrCriteriaReportInvalid)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxCriteriaReportFileBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("%w: %v", reports.ErrCriteriaReportInvalid, err)
	}
	return nil
}

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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ResolveFreshRetryAnchor resolves the git anchor for a fresh operator retry.
func (s *Service) ResolveFreshRetryAnchor(ctx context.Context, workingDir, parentCycleID string) (string, error) {
	return s.resolveFreshRetryAnchor(ctx, workingDir, parentCycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ResetHardClean runs git reset --hard and git clean -fd (test seam).
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
