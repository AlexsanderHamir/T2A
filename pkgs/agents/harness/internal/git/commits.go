package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const (
	// ExecuteNoCommitsReason is recorded when execute produced no commits in range.
	ExecuteNoCommitsReason = "execute_no_commits"
	// ExecuteUncommittedWorkReason is recorded when the worktree is dirty after execute.
	ExecuteUncommittedWorkReason = "execute_uncommitted_work"
	// ExecuteInvalidCommitReason is recorded when criteria-report SHAs are invalid.
	ExecuteInvalidCommitReason = "execute_invalid_commit"
	// ExecuteRewrittenHistoryReason is recorded when stored SHAs disappear from ancestry.
	ExecuteRewrittenHistoryReason = "execute_rewritten_history"

	RetryResetAnchorMissing = "retry_reset_anchor_missing"
	RetryGitResetFailed     = "retry_git_reset_failed"
)

// ExecuteCommitIngestOutcome summarizes commit observe/admit after execute.
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

type commitReport struct {
	SHA    string `json:"sha"`
	Branch string `json:"branch"`
}

// MatchReportedSHAInAncestry maps an agent-reported SHA to canonical cycle ancestry.
func MatchReportedSHAInAncestry(reported string, ancestry []string) (string, error) {
	reported = strings.ToLower(strings.TrimSpace(reported))
	if reported == "" {
		return "", fmt.Errorf("%w: empty reported sha", domain.ErrInvalidInput)
	}
	var prefixMatches []string
	for _, full := range ancestry {
		full = strings.TrimSpace(full)
		if full == "" {
			continue
		}
		lower := strings.ToLower(full)
		if lower == reported {
			return full, nil
		}
		if strings.HasPrefix(lower, reported) {
			prefixMatches = append(prefixMatches, full)
		}
	}
	switch len(prefixMatches) {
	case 1:
		return prefixMatches[0], nil
	case 0:
		return "", fmt.Errorf("%w: reported sha not in cycle ancestry", domain.ErrInvalidInput)
	default:
		return "", fmt.Errorf("%w: ambiguous abbreviated sha %q within cycle ancestry", domain.ErrInvalidInput, reported)
	}
}

func (s *Service) revListRange(ctx context.Context, worktree, baseSHA string) ([]string, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	if baseSHA == "" {
		return nil, fmt.Errorf("empty base sha")
	}
	out, err := s.repo().Run(ctx, worktree, "rev-list", "--reverse", baseSHA+"..HEAD")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

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

func buildReportedBranchMap(reported []commitReport, shas []string, cycleID string) (map[string]string, error) {
	reportedMap := make(map[string]string, len(reported))
	for _, r := range reported {
		raw := strings.TrimSpace(r.SHA)
		if raw == "" {
			continue
		}
		full, err := MatchReportedSHAInAncestry(raw, shas)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidInput) && strings.Contains(err.Error(), "not in cycle ancestry") {
				slog.Warn("ignoring criteria-report commit outside cycle ancestry",
					"cmd", logCmd, "operation", "agent.harness.git.buildReportedBranchMap",
					"cycle_id", cycleID, "reported_sha", raw)
				continue
			}
			return nil, err
		}
		reportedMap[full] = strings.TrimSpace(r.Branch)
	}
	return reportedMap, nil
}

func (s *Service) resolvePhaseCommits(ctx context.Context, g phaseContext, reported []commitReport, cycleID string) ([]store.CycleCommitEntry, error) {
	shas, err := s.revListRange(ctx, g.Worktree, g.CycleBaseSHA)
	if err != nil {
		return nil, err
	}
	reportedMap, err := buildReportedBranchMap(reported, shas, cycleID)
	if err != nil {
		return nil, err
	}
	if len(shas) == 0 {
		return nil, nil
	}
	out := make([]store.CycleCommitEntry, 0, len(shas))
	for i, sha := range shas {
		sha = strings.TrimSpace(sha)
		msg, at, err := s.commitDetails(ctx, g.Worktree, sha)
		if err != nil {
			return nil, err
		}
		branch := reportedMap[sha]
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
		})
	}
	return out, nil
}

func (s *Service) commitExists(ctx context.Context, worktree, sha string) bool {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return false
	}
	_, err := s.repo().Run(ctx, worktree, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// BuildInheritedCommitEntries copies parent-cycle commits when resume made no new commits.
func (s *Service) BuildInheritedCommitEntries(
	ctx context.Context,
	g phaseContext,
	inherited []domain.TaskCycleCommit,
	execPhaseSeq int64,
) ([]store.CycleCommitEntry, error) {
	if len(inherited) == 0 {
		return nil, nil
	}
	out := make([]store.CycleCommitEntry, 0, len(inherited))
	seq := int64(0)
	for _, c := range inherited {
		sha := strings.TrimSpace(c.SHA)
		if sha == "" {
			continue
		}
		if !s.commitExists(ctx, g.Worktree, sha) {
			return nil, fmt.Errorf("%w: inherited commit %s no longer in repository", domain.ErrInvalidInput, sha)
		}
		seq++
		msg := c.Message
		at := c.CommittedAt
		if refreshedMsg, refreshedAt, err := s.commitDetails(ctx, g.Worktree, sha); err == nil {
			if refreshedMsg != "" {
				msg = refreshedMsg
			}
			if !refreshedAt.IsZero() {
				at = refreshedAt
			}
		}
		branch := strings.TrimSpace(c.Branch)
		if branch == "" {
			branch, _ = s.branchContaining(ctx, g.Worktree, sha)
		}
		if branch == "" {
			branch = g.BaseBranch
		}
		repo := strings.TrimSpace(c.Repo)
		if repo == "" {
			repo = g.Repo
		}
		worktree := strings.TrimSpace(c.Worktree)
		if worktree == "" {
			worktree = g.Worktree
		}
		out = append(out, store.CycleCommitEntry{
			Seq:           seq,
			Repo:          repo,
			Worktree:      worktree,
			Branch:        branch,
			SHA:           sha,
			CommittedAt:   at,
			Message:       msg,
			PhaseSeq:      execPhaseSeq,
			Status:        domain.CommitInherited,
			SourceCycleID: strings.TrimSpace(c.CycleID),
		})
	}
	return out, nil
}

func (s *Service) evaluateExecuteCommitGates(
	ctx context.Context,
	snap PhaseSnapshot,
	cycleID string,
	entries []store.CycleCommitEntry,
) (failReason string, err error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.evaluateExecuteCommitGates",
		"cycle_id", cycleID, "entry_count", len(entries))
	dirty, err := WorkingTreeDirty(ctx, s.repo(), snap.Worktree)
	if err != nil {
		return "", err
	}
	if dirty {
		return ExecuteUncommittedWorkReason, nil
	}
	stored, err := s.store.ListCommitsForCycle(ctx, cycleID)
	if err != nil {
		return "", err
	}
	current := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		current[e.SHA] = struct{}{}
	}
	for _, row := range stored {
		if _, ok := current[row.SHA]; !ok {
			return ExecuteRewrittenHistoryReason, nil
		}
	}
	return "", nil
}

// AssignCommitAdmissionStatuses sets observe/eligible status from gate outcome.
func AssignCommitAdmissionStatuses(entries []store.CycleCommitEntry, failReason string) {
	assignCommitAdmissionStatuses(entries, failReason)
}

func assignCommitAdmissionStatuses(entries []store.CycleCommitEntry, failReason string) {
	for i := range entries {
		if failReason != "" {
			entries[i].Status = domain.CommitObserved
			entries[i].GateReason = failReason
			continue
		}
		if entries[i].Status == domain.CommitInherited {
			entries[i].Status = domain.CommitEligible
			entries[i].GateReason = ""
			continue
		}
		entries[i].Status = domain.CommitEligible
		entries[i].GateReason = ""
	}
}

// ResolvePhaseCommitsFromReports resolves commits using criteria-report SHA hints.
func ResolvePhaseCommitsFromReports(ctx context.Context, s *Service, snap PhaseSnapshot, reported []CommitReport, cycleID string) ([]store.CycleCommitEntry, error) {
	reports := make([]commitReport, len(reported))
	for i, r := range reported {
		reports[i] = commitReport(r)
	}
	g := phaseContext{
		Repo:         snap.Repo,
		Worktree:     snap.Worktree,
		BaseSHA:      snap.BaseSHA,
		CycleBaseSHA: snap.CycleBaseSHA,
		BaseBranch:   snap.BaseBranch,
	}
	return s.resolvePhaseCommits(ctx, g, reports, cycleID)
}

// CommitReport is a criteria-report commit entry.
type CommitReport struct {
	SHA    string `json:"sha"`
	Branch string `json:"branch"`
}

// PhaseContext carries git anchors for commit resolution.
type PhaseContext struct {
	Repo         string
	Worktree     string
	BaseSHA      string
	CycleBaseSHA string
	BaseBranch   string
}

func phaseContextFromSnapshot(snap PhaseSnapshot) phaseContext {
	return phaseContext{
		Repo:         snap.Repo,
		Worktree:     snap.Worktree,
		BaseSHA:      snap.BaseSHA,
		CycleBaseSHA: snap.CycleBaseSHA,
		BaseBranch:   snap.BaseBranch,
	}
}

// IngestExecuteCommits observes git ancestry after execute, upserts commits, evaluates gates.
func (s *Service) IngestExecuteCommits(
	ctx context.Context,
	taskID string,
	cycle *domain.TaskCycle,
	execPhaseSeq int64,
	snap PhaseSnapshot,
	inherited []domain.TaskCycleCommit,
	retryMode domain.RetryMode,
	publish func(taskID, cycleID string),
) (ExecuteCommitIngestOutcome, error) {
	if snap.Skipped {
		return ExecuteCommitIngestOutcome{}, nil
	}
	reported, err := s.parseCommitReports(s.reportDir, cycle.ID)
	if err != nil {
		return ExecuteCommitIngestOutcome{FailReason: ExecuteInvalidCommitReason}, err
	}
	g := phaseContextFromSnapshot(snap)
	entries, err := s.resolvePhaseCommits(ctx, g, reported, cycle.ID)
	if err != nil {
		return ExecuteCommitIngestOutcome{FailReason: ExecuteInvalidCommitReason}, err
	}
	if len(entries) == 0 && retryMode == domain.RetryResume && len(inherited) > 0 {
		entries, err = s.BuildInheritedCommitEntries(ctx, g, inherited, execPhaseSeq)
		if err != nil {
			return ExecuteCommitIngestOutcome{FailReason: ExecuteInvalidCommitReason}, err
		}
		if len(entries) > 0 {
			slog.Info("agent harness inherited parent commits for resume attempt",
				"cmd", logCmd, "operation", "agent.harness.git.IngestExecuteCommits.inherit",
				"cycle_id", cycle.ID, "commit_count", len(entries))
		}
	}
	if len(entries) == 0 {
		return ExecuteCommitIngestOutcome{FailReason: ExecuteNoCommitsReason}, nil
	}
	for i := range entries {
		if entries[i].PhaseSeq == 0 {
			entries[i].PhaseSeq = execPhaseSeq
		}
	}
	failReason, err := s.evaluateExecuteCommitGates(ctx, snap, cycle.ID, entries)
	if err != nil {
		return ExecuteCommitIngestOutcome{}, err
	}
	assignCommitAdmissionStatuses(entries, failReason)
	if err := s.store.UpsertCycleCommits(ctx, taskID, cycle.ID, entries); err != nil {
		return ExecuteCommitIngestOutcome{}, err
	}
	keepSHAs := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		keepSHAs[e.SHA] = struct{}{}
	}
	if err := s.store.MarkCycleCommitsSuperseded(ctx, cycle.ID, keepSHAs); err != nil {
		return ExecuteCommitIngestOutcome{}, err
	}
	if publish != nil {
		publish(taskID, cycle.ID)
	}
	if failReason != "" {
		return ExecuteCommitIngestOutcome{FailReason: failReason, CommitCount: len(entries)}, nil
	}
	return ExecuteCommitIngestOutcome{CommitCount: len(entries)}, nil
}

// PriorCycleBaseSHA reads cycle_base_sha from the earliest prior execute phase.
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
