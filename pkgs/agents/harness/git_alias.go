package harness

import (
	"context"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const (
	executeNoCommitsReason            = git.ExecuteNoCommitsReason
	executeUncommittedWorkReason      = git.ExecuteUncommittedWorkReason
	executeInvalidCommitReason        = git.ExecuteInvalidCommitReason
	executeRewrittenHistoryReason     = git.ExecuteRewrittenHistoryReason
	verifyTamperedReason              = git.VerifyTamperedReason
	verifyIntegrityCheckTimeoutReason = git.VerifyIntegrityCheckTimeoutReason
	retryResetAnchorMissing           = git.RetryResetAnchorMissing
	retryGitResetFailed               = git.RetryGitResetFailed
)

type executeCommitIngestOutcome = git.ExecuteCommitIngestOutcome

func (h *Harness) gitSvc() *git.Service {
	if h.git == nil {
		h.git = git.NewService(h.store, git.NewExecRepo(), h.opts.ReportDir)
	}
	h.git.SetReportDir(h.opts.ReportDir)
	return h.git
}

func (h *Harness) priorCycleBaseSHA(ctx context.Context, cycleID string, currentPhaseSeq int64) (string, error) {
	return h.gitSvc().PriorCycleBaseSHA(ctx, cycleID, currentPhaseSeq)
}

func (h *Harness) ingestExecuteCommits(
	ctx context.Context,
	taskID string,
	cycle *domain.TaskCycle,
	execPhaseSeq int64,
	snap git.PhaseSnapshot,
	inherited []domain.TaskCycleCommit,
	retryMode domain.RetryMode,
) (executeCommitIngestOutcome, error) {
	return h.gitSvc().IngestExecuteCommits(ctx, taskID, cycle, execPhaseSeq, snap, inherited, retryMode, h.publish)
}

func (h *Harness) gitResetForFreshRetry(ctx context.Context, parentCycleID string) (git.FreshRetryResetOutcome, error) {
	return h.gitSvc().ResetForFreshRetry(ctx, h.opts.WorkingDir, parentCycleID)
}

func (h *Harness) checkVerifyIntegrity(ctx context.Context, cycleID string, pre git.IntegritySnapshot, preErr error) (bool, string) {
	return git.CheckVerifyIntegrity(ctx, h.gitSvc().Repo(), h.opts.WorkingDir, cycleID, pre, preErr)
}

func captureExecuteGitSnapshot(ctx context.Context, repo git.GitRepo, repoRoot, workdir, priorCycleBase string) (git.PhaseSnapshot, error) {
	return git.CaptureExecuteGitSnapshot(ctx, repo, repoRoot, workdir, priorCycleBase)
}

func captureIntegritySnapshot(ctx context.Context, repo git.GitRepo, workingDir string) (git.IntegritySnapshot, error) {
	return git.CaptureIntegritySnapshot(ctx, repo, workingDir)
}

func mergeRunnerDetailsWithGit(baseDetails []byte, snap git.PhaseSnapshot, commitCount int) []byte {
	return git.MergeRunnerDetailsWithGit(baseDetails, snap, commitCount)
}

func formatGitContextForPrompt(commits []domain.TaskCycleCommit) string {
	return git.FormatGitContextForPrompt(commits)
}

func formatKnownCommitsForResume(commits []domain.TaskCycleCommit) string {
	return git.FormatKnownCommitsForResume(commits)
}

func gitCycleBaseFromPhaseDetails(details []byte) string {
	return git.CycleBaseFromPhaseDetails(details)
}
