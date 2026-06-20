package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/reports"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const (
	// ExecuteNoCommitsReason is recorded when execute produced no commits in range.
	ExecuteNoCommitsReason = "execute_no_commits"
	// ExecuteUncommittedWorkReason is recorded when the worktree is dirty after execute.
	ExecuteUncommittedWorkReason = "execute_uncommitted_work"
	// ExecuteInvalidCommitReason is recorded when inherited or resolved commits are invalid.
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Git sub-step; operation trace is emitted by IngestExecuteCommits."
func (s *Service) resolvePhaseCommits(ctx context.Context, g phaseContext) ([]store.CycleCommitEntry, error) {
	shas, err := s.revListRange(ctx, g.Worktree, g.CycleBaseSHA)
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
		branch, _ := s.branchContaining(ctx, g.Worktree, sha)
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Service) commitExists(ctx context.Context, worktree, sha string) bool {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return false
	}
	_, err := s.repo().Run(ctx, worktree, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// BuildInheritedCommitEntries copies parent-cycle commits when resume made no new commits.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
		if len(entries) == 0 {
			return ExecuteUncommittedWorkReason, nil
		}
		slog.Warn("working tree dirty after execute but cycle has commits; admitting commits",
			"cmd", logCmd, "operation", "agent.harness.git.evaluateExecuteCommitGates.dirty_with_commits",
			"cycle_id", cycleID, "commit_count", len(entries))
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
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func AssignCommitAdmissionStatuses(entries []store.CycleCommitEntry, failReason string) {
	assignCommitAdmissionStatuses(entries, failReason)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
	g := phaseContextFromSnapshot(snap)
	warnStrictCriteriaReportDecode(s.reportDir, cycle.ID)
	entries, err := s.resolvePhaseCommits(ctx, g)
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

const maxCriteriaReportProbeBytes = 256 * 1024

// warnStrictCriteriaReportDecode logs criteria-report strict-decode issues without
// blocking commit ingest — git rev-list remains the sole source of truth (ADR-0016).
//
//funclogmeasure:skip category=hot-path reason="Non-fatal probe; IngestExecuteCommits emits the operation trace."
func warnStrictCriteriaReportDecode(reportDir, cycleID string) {
	reportDir = strings.TrimSpace(reportDir)
	cycleID = strings.TrimSpace(cycleID)
	if reportDir == "" || cycleID == "" {
		return
	}
	path := reports.CriteriaReportPath(reportDir, cycleID)
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		slog.Warn("criteria report probe failed; ingesting commits from git only",
			"cmd", logCmd, "operation", "agent.harness.git.IngestExecuteCommits.criteria_report",
			"cycle_id", cycleID, "err", err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Warn("criteria report probe failed; ingesting commits from git only",
			"cmd", logCmd, "operation", "agent.harness.git.IngestExecuteCommits.criteria_report",
			"cycle_id", cycleID, "err", "symlink not permitted")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		slog.Warn("criteria report probe failed; ingesting commits from git only",
			"cmd", logCmd, "operation", "agent.harness.git.IngestExecuteCommits.criteria_report",
			"cycle_id", cycleID, "err", err)
		return
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxCriteriaReportProbeBytes))
	dec.DisallowUnknownFields()
	var rep struct {
		SchemaVersion int               `json:"schema_version"`
		Criteria      []json.RawMessage `json:"criteria"`
		Commits       []json.RawMessage `json:"commits,omitempty"`
	}
	if err := dec.Decode(&rep); err != nil {
		slog.Warn("criteria report parse failed; ingesting commits from git only",
			"cmd", logCmd, "operation", "agent.harness.git.IngestExecuteCommits.criteria_report",
			"cycle_id", cycleID, "err", err)
	}
}
