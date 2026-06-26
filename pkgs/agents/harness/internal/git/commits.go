package git

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

const (
	// ExecuteInvalidCommitReason is recorded when claimed commits cannot be resolved in git.
	ExecuteInvalidCommitReason = "execute_invalid_commit"

	RetryResetAnchorMissing = "retry_reset_anchor_missing"
	RetryGitResetFailed     = "retry_git_reset_failed"
)

// ErrRetryResetAnchorMissing is returned when a fresh retry cannot resolve a git reset anchor.
var ErrRetryResetAnchorMissing = errors.New(RetryResetAnchorMissing)

// ExecuteCommitIngestOutcome summarizes commit indexing after execute.
type ExecuteCommitIngestOutcome struct {
	FailReason  string
	CommitCount int
}

// FreshRetryResetOutcome reports whether fresh-retry git reset was skipped.
type FreshRetryResetOutcome struct {
	Skipped bool
}

type phaseContext struct {
	Repo         string
	Worktree     string
	BaseSHA      string
	CycleBaseSHA string
	BaseBranch   string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) commitDetails(ctx context.Context, worktree, sha string) (message string, committedAt time.Time, err error) {
	out, err := s.repo().Run(ctx, worktree, "log", "-1", "--format=%s%n%ci", sha)
	if err != nil {
		return "", time.Time{}, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return "", time.Time{}, nil
	}
	msg := strings.TrimSpace(lines[0])
	if len(lines) < 2 {
		return msg, time.Time{}, nil
	}
	ts, parseErr := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(lines[1]))
	if parseErr != nil {
		ts, parseErr = time.Parse(time.RFC3339, strings.TrimSpace(lines[1]))
	}
	if parseErr != nil {
		return msg, time.Time{}, nil
	}
	return msg, ts.UTC(), nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) branchContaining(ctx context.Context, worktree, sha string) (string, error) {
	out, err := s.repo().Run(ctx, worktree, "branch", "--contains", sha, "--format=%(refname:short)")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) commitExists(ctx context.Context, worktree, sha string) bool {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return false
	}
	_, err := s.repo().Run(ctx, worktree, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) commitInRange(ctx context.Context, worktree, cycleBaseSHA, sha string) bool {
	cycleBaseSHA = strings.TrimSpace(cycleBaseSHA)
	sha = strings.TrimSpace(sha)
	if cycleBaseSHA == "" || sha == "" {
		return false
	}
	baseAncestor, err := s.repo().Run(ctx, worktree, "merge-base", "--is-ancestor", cycleBaseSHA, sha)
	if err != nil || strings.TrimSpace(baseAncestor) != "yes" {
		return false
	}
	if cycleBaseSHA == sha {
		return false
	}
	headAncestor, err := s.repo().Run(ctx, worktree, "merge-base", "--is-ancestor", sha, "HEAD")
	return err == nil && strings.TrimSpace(headAncestor) == "yes"
}

//funclogmeasure:skip category=hot-path reason="Git sub-step; operation trace is emitted by IngestExecuteCommits."
func (s *Service) resolveClaimedCommits(
	ctx context.Context,
	g phaseContext,
	claims []reports.CriteriaCommitClaim,
	execPhaseSeq int64,
) ([]store.CycleCommitEntry, error) {
	if len(claims) == 0 {
		return nil, nil
	}
	out := make([]store.CycleCommitEntry, 0, len(claims))
	for i, claim := range claims {
		sha := strings.TrimSpace(claim.SHA)
		if !s.commitExists(ctx, g.Worktree, sha) {
			return nil, fmt.Errorf("%w: commit %s not found in repository", domain.ErrInvalidInput, sha)
		}
		if g.CycleBaseSHA != "" && !s.commitInRange(ctx, g.Worktree, g.CycleBaseSHA, sha) {
			slog.Warn("claimed commit outside cycle_base_sha..HEAD; indexing anyway",
				"cmd", calltrace.LogCmd, "operation", "agent.harness.git.resolveClaimedCommits.out_of_range",
				"sha", sha, "cycle_base_sha", g.CycleBaseSHA)
		}
		msg, at, err := s.commitDetails(ctx, g.Worktree, sha)
		if err != nil {
			return nil, err
		}
		branch := strings.TrimSpace(claim.Branch)
		if branch == "" {
			branch, _ = s.branchContaining(ctx, g.Worktree, sha)
		}
		if branch == "" {
			branch = g.BaseBranch
		}
		out = append(out, store.CycleCommitEntry{
			Seq:         int64(i + 1),
			Repo:        g.Repo,
			Worktree:    g.Worktree,
			Branch:      branch,
			SHA:         sha,
			CommittedAt: at,
			Message:     msg,
			PhaseSeq:    execPhaseSeq,
		})
	}
	return out, nil
}

func (s *Service) warnMissingIndexedCommits(ctx context.Context, taskID string, worktree string) {
	prior, err := s.store.ListCommitsForTask(ctx, taskID)
	if err != nil {
		return
	}
	for _, row := range prior {
		if !s.commitExists(ctx, worktree, row.SHA) {
			slog.Warn("indexed commit no longer in repository",
				"cmd", calltrace.LogCmd, "operation", "agent.harness.git.IngestExecuteCommits.missing_prior_sha",
				"task_id", taskID, "sha", row.SHA)
		}
	}
}

// PhaseContext carries git anchors for commit resolution.
type PhaseContext struct {
	Repo         string
	Worktree     string
	BaseSHA      string
	CycleBaseSHA string
	BaseBranch   string
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func phaseContextFromSnapshot(snap PhaseSnapshot) phaseContext {
	return phaseContext{
		Repo:         snap.Repo,
		Worktree:     snap.Worktree,
		BaseSHA:      snap.BaseSHA,
		CycleBaseSHA: snap.CycleBaseSHA,
		BaseBranch:   snap.BaseBranch,
	}
}

// IngestExecuteCommits indexes agent-declared commits from criteria-report.json.
func (s *Service) IngestExecuteCommits(
	ctx context.Context,
	taskID string,
	cycle *domain.TaskCycle,
	execPhaseSeq int64,
	snap PhaseSnapshot,
	publish func(taskID, cycleID string),
) (ExecuteCommitIngestOutcome, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.harness.git.IngestExecuteCommits",
		"task_id", taskID, "cycle_id", cycle.ID, "phase_seq", execPhaseSeq)
	if snap.Skipped {
		return ExecuteCommitIngestOutcome{}, nil
	}
	g := phaseContextFromSnapshot(snap)
	s.warnMissingIndexedCommits(ctx, taskID, g.Worktree)

	claims, err := reports.ParseCriteriaReportCommits(s.reportDir, cycle.ID)
	if err != nil {
		if errors.Is(err, reports.ErrCriteriaReportInvalid) {
			return ExecuteCommitIngestOutcome{FailReason: ExecuteInvalidCommitReason}, err
		}
		return ExecuteCommitIngestOutcome{}, err
	}
	entries, err := s.resolveClaimedCommits(ctx, g, claims, execPhaseSeq)
	if err != nil {
		return ExecuteCommitIngestOutcome{FailReason: ExecuteInvalidCommitReason}, err
	}
	if len(entries) == 0 {
		return ExecuteCommitIngestOutcome{}, nil
	}
	if err := s.store.UpsertCycleCommits(ctx, taskID, cycle.ID, entries); err != nil {
		return ExecuteCommitIngestOutcome{}, err
	}
	if publish != nil {
		publish(taskID, cycle.ID)
	}
	return ExecuteCommitIngestOutcome{CommitCount: len(entries)}, nil
}

// PriorCycleBaseSHA reads cycle_base_sha from the earliest prior execute phase.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) PriorCycleBaseSHA(ctx context.Context, cycleID string, currentPhaseSeq int64) (string, error) {
	phases, err := s.store.ListPhasesForCycle(ctx, cycleID)
	if err != nil {
		return "", err
	}
	var first *domain.TaskCyclePhase
	for i := range phases {
		p := &phases[i]
		if p.Phase != domain.PhaseExecute || p.PhaseSeq >= currentPhaseSeq {
			continue
		}
		if first == nil || p.PhaseSeq < first.PhaseSeq {
			first = p
		}
	}
	if first == nil {
		return "", nil
	}
	return CycleBaseFromPhaseDetails(first.DetailsJSON), nil
}
