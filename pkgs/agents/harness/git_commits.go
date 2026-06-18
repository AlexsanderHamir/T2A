package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const (
	executeNoCommitsReason        = "execute_no_commits"
	executeUncommittedWorkReason  = "execute_uncommitted_work"
	executeInvalidCommitReason    = "execute_invalid_commit"
	executeRewrittenHistoryReason = "execute_rewritten_history"
	gitSnapshotProbeTimeout       = 30 * time.Second
)

// gitPhaseSnapshot captures repo/worktree/HEAD anchors at execute start.
type gitPhaseSnapshot struct {
	Skipped      bool
	Repo         string
	Worktree     string
	BaseSHA      string
	CycleBaseSHA string
	BaseBranch   string
	CapturedAt   time.Time
}

type gitPhaseContext struct {
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

// captureExecuteGitSnapshot records git anchors for this execute phase.
// When workdir is not a git repo, Skipped is true and ingest/gates are bypassed.
func captureExecuteGitSnapshot(ctx context.Context, repoRoot, workdir, priorCycleBase string) (gitPhaseSnapshot, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.captureExecuteGitSnapshot",
		"repo_root", repoRoot, "workdir", workdir)
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return gitPhaseSnapshot{Skipped: true}, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, gitSnapshotProbeTimeout)
	defer cancel()

	head, err := runGit(probeCtx, workdir, "rev-parse", "HEAD")
	if err != nil {
		if isNotAGitRepoErr(err) {
			return gitPhaseSnapshot{Skipped: true}, nil
		}
		return gitPhaseSnapshot{}, err
	}
	worktree, err := runGit(probeCtx, workdir, "rev-parse", "--show-toplevel")
	if err != nil {
		if isNotAGitRepoErr(err) {
			return gitPhaseSnapshot{Skipped: true}, nil
		}
		return gitPhaseSnapshot{}, err
	}
	branch, err := runGit(probeCtx, workdir, "branch", "--show-current")
	if err != nil && !isNotAGitRepoErr(err) {
		return gitPhaseSnapshot{}, err
	}
	cycleBase := strings.TrimSpace(priorCycleBase)
	if cycleBase == "" {
		cycleBase = head
	}
	return gitPhaseSnapshot{
		Repo:         strings.TrimSpace(repoRoot),
		Worktree:     worktree,
		BaseSHA:      head,
		CycleBaseSHA: cycleBase,
		BaseBranch:   strings.TrimSpace(branch),
		CapturedAt:   time.Now().UTC(),
	}, nil
}

func (h *Harness) priorCycleBaseSHA(ctx context.Context, cycleID string, currentPhaseSeq int64) (string, error) {
	phases, err := h.store.ListPhasesForCycle(ctx, cycleID)
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
	return gitCycleBaseFromPhaseDetails(first.DetailsJSON), nil
}

func gitCycleBaseFromPhaseDetails(details []byte) string {
	if len(details) == 0 {
		return ""
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(details, &root); err != nil {
		return ""
	}
	raw, ok := root["git"]
	if !ok {
		return ""
	}
	var git struct {
		CycleBaseSHA string `json:"cycle_base_sha"`
		BaseSHA      string `json:"base_sha"`
	}
	if err := json.Unmarshal(raw, &git); err != nil {
		return ""
	}
	if v := strings.TrimSpace(git.CycleBaseSHA); v != "" {
		return v
	}
	return strings.TrimSpace(git.BaseSHA)
}

func gitSnapshotToMap(s gitPhaseSnapshot, commitCount int) map[string]any {
	m := map[string]any{
		"repo":           s.Repo,
		"worktree":       s.Worktree,
		"base_sha":       s.BaseSHA,
		"cycle_base_sha": s.CycleBaseSHA,
		"base_branch":    s.BaseBranch,
		"captured_at":    s.CapturedAt.Format(time.RFC3339),
	}
	if commitCount > 0 {
		m["commit_count"] = commitCount
	}
	if s.Skipped {
		m["skipped"] = true
	}
	return m
}

// mergeRunnerDetailsWithGit attaches git snapshot metadata to execute phase details.
func mergeRunnerDetailsWithGit(baseDetails []byte, snap gitPhaseSnapshot, commitCount int) []byte {
	if snap.Skipped && commitCount == 0 {
		if len(baseDetails) == 0 {
			return baseDetails
		}
		return baseDetails
	}
	root := map[string]any{}
	if len(baseDetails) > 0 {
		_ = json.Unmarshal(baseDetails, &root)
	}
	root["git"] = gitSnapshotToMap(snap, commitCount)
	out, err := json.Marshal(root)
	if err != nil {
		return baseDetails
	}
	return out
}

func parseCommitReports(reportDir, cycleID string) ([]commitReport, error) {
	path := criteriaReportPath(reportDir, cycleID)
	var rep struct {
		Criteria []criteriaReportEntry `json:"criteria"`
		Commits  []commitReport        `json:"commits"`
	}
	if err := readJSONFile(path, &rep); err != nil {
		if errors.Is(err, ErrCriteriaReportMissing) {
			return nil, nil
		}
		return nil, err
	}
	return rep.Commits, nil
}

func gitRevListRange(ctx context.Context, worktree, baseSHA string) ([]string, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	if baseSHA == "" {
		return nil, fmt.Errorf("empty base sha")
	}
	out, err := runGit(ctx, worktree, "rev-list", "--reverse", baseSHA+"..HEAD")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func gitCommitDetails(ctx context.Context, worktree, sha string) (message string, committedAt time.Time, err error) {
	out, err := runGit(ctx, worktree, "log", "-1", "--format=%s%n%ci", sha)
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

func gitBranchContaining(ctx context.Context, worktree, sha string) (string, error) {
	out, err := runGit(ctx, worktree, "branch", "--contains", sha, "--format=%(refname:short)")
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

func gitWorkingTreeDirty(ctx context.Context, worktree string) (bool, error) {
	snap, err := captureIntegritySnapshot(ctx, worktree)
	if err != nil {
		return false, err
	}
	if snap.notGitRepo {
		return false, nil
	}
	return len(snap.changed) > 0, nil
}

func resolvePhaseCommits(ctx context.Context, g gitPhaseContext, reported []commitReport) ([]store.CycleCommitEntry, error) {
	shas, err := gitRevListRange(ctx, g.Worktree, g.CycleBaseSHA)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(shas))
	for _, sha := range shas {
		set[strings.TrimSpace(sha)] = struct{}{}
	}
	reportedMap := make(map[string]string, len(reported))
	for _, r := range reported {
		sha := strings.TrimSpace(r.SHA)
		if sha == "" {
			continue
		}
		if _, ok := set[sha]; !ok {
			return nil, fmt.Errorf("%w: reported sha not in cycle ancestry", domain.ErrInvalidInput)
		}
		reportedMap[sha] = strings.TrimSpace(r.Branch)
	}
	if len(shas) == 0 {
		return nil, nil
	}
	out := make([]store.CycleCommitEntry, 0, len(shas))
	for i, sha := range shas {
		sha = strings.TrimSpace(sha)
		msg, at, err := gitCommitDetails(ctx, g.Worktree, sha)
		if err != nil {
			return nil, err
		}
		branch := reportedMap[sha]
		if branch == "" {
			branch, _ = gitBranchContaining(ctx, g.Worktree, sha)
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

type executeCommitIngestOutcome struct {
	FailReason  string
	CommitCount int
}

// ingestExecuteCommits validates git state after a successful execute run
// and upserts task_cycle_commits before CompletePhase(execute).
func (h *Harness) ingestExecuteCommits(
	ctx context.Context,
	taskID string,
	cycle *domain.TaskCycle,
	execPhaseSeq int64,
	snap gitPhaseSnapshot,
) (executeCommitIngestOutcome, error) {
	if snap.Skipped {
		return executeCommitIngestOutcome{}, nil
	}
	reported, err := parseCommitReports(h.opts.ReportDir, cycle.ID)
	if err != nil {
		return executeCommitIngestOutcome{FailReason: executeInvalidCommitReason}, err
	}
	g := gitPhaseContext{
		Repo:         snap.Repo,
		Worktree:     snap.Worktree,
		BaseSHA:      snap.BaseSHA,
		CycleBaseSHA: snap.CycleBaseSHA,
		BaseBranch:   snap.BaseBranch,
	}
	entries, err := resolvePhaseCommits(ctx, g, reported)
	if err != nil {
		return executeCommitIngestOutcome{FailReason: executeInvalidCommitReason}, err
	}
	if len(entries) == 0 {
		return executeCommitIngestOutcome{FailReason: executeNoCommitsReason}, nil
	}
	dirty, err := gitWorkingTreeDirty(ctx, snap.Worktree)
	if err != nil {
		return executeCommitIngestOutcome{}, err
	}
	if dirty {
		return executeCommitIngestOutcome{FailReason: executeUncommittedWorkReason}, nil
	}
	stored, err := h.store.ListCommitsForCycle(ctx, cycle.ID)
	if err != nil {
		return executeCommitIngestOutcome{}, err
	}
	current := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		current[e.SHA] = struct{}{}
	}
	for _, row := range stored {
		if _, ok := current[row.SHA]; !ok {
			return executeCommitIngestOutcome{FailReason: executeRewrittenHistoryReason}, nil
		}
	}
	for i := range entries {
		entries[i].PhaseSeq = execPhaseSeq
	}
	if err := h.store.UpsertCycleCommits(ctx, taskID, cycle.ID, entries); err != nil {
		return executeCommitIngestOutcome{}, err
	}
	return executeCommitIngestOutcome{CommitCount: len(entries)}, nil
}

func formatGitContextForPrompt(commits []domain.TaskCycleCommit) string {
	if len(commits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Git context (worker-indexed)\n\n")
	first := commits[0]
	if first.Repo != "" {
		b.WriteString("Repo:     ")
		b.WriteString(first.Repo)
		b.WriteByte('\n')
	}
	if first.Worktree != "" && first.Worktree != first.Repo {
		b.WriteString("Worktree: ")
		b.WriteString(first.Worktree)
		b.WriteByte('\n')
	}
	if first.Branch != "" {
		b.WriteString("Branch:   ")
		b.WriteString(first.Branch)
		b.WriteByte('\n')
	}
	b.WriteString("Commits:\n")
	for _, c := range commits {
		short := c.SHA
		if len(short) > 7 {
			short = short[:7]
		}
		ts := c.CommittedAt.UTC().Format(time.RFC3339)
		b.WriteString(fmt.Sprintf("%d. %s… @ %s — %q\n", c.Seq, short, ts, c.Message))
	}
	b.WriteString(fmt.Sprintf("commit_count=%d\n\n", len(commits)))
	return b.String()
}

func formatKnownCommitsForResume(commits []domain.TaskCycleCommit) string {
	if len(commits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Known commits already indexed for this cycle:\n")
	for _, c := range commits {
		short := c.SHA
		if len(short) > 12 {
			short = short[:12]
		}
		b.WriteString(fmt.Sprintf("- %s — %s\n", short, c.Message))
	}
	b.WriteByte('\n')
	return b.String()
}
