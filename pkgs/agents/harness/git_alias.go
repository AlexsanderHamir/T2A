package harness

import (
	"context"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

const (
	executeInvalidCommitReason        = git.ExecuteInvalidCommitReason
	verifyTamperedReason              = git.VerifyTamperedReason
	verifyIntegrityCheckTimeoutReason = git.VerifyIntegrityCheckTimeoutReason
	retryResetAnchorMissing           = git.RetryResetAnchorMissing
	retryGitResetFailed               = git.RetryGitResetFailed
)

type executeCommitIngestOutcome = git.ExecuteCommitIngestOutcome

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) gitSvc() *git.Service {
	if h.git == nil {
		h.git = git.NewService(h.store, git.NewExecRepo(), h.opts.ReportDir)
	}
	h.git.SetReportDir(h.opts.ReportDir)
	return h.git
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) priorCycleBaseSHA(ctx context.Context, cycleID string, currentPhaseSeq int64) (string, error) {
	return h.gitSvc().PriorCycleBaseSHA(ctx, cycleID, currentPhaseSeq)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) ingestExecuteCommits(
	ctx context.Context,
	taskID string,
	cycle *domain.TaskCycle,
	execPhaseSeq int64,
	snap git.PhaseSnapshot,
) (executeCommitIngestOutcome, error) {
	return h.gitSvc().IngestExecuteCommits(ctx, taskID, cycle, execPhaseSeq, snap, h.publish)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) gitResetForFreshRetry(ctx context.Context, parentCycleID string) (git.FreshRetryResetOutcome, error) {
	return h.gitSvc().ResetForFreshRetry(ctx, h.opts.WorkingDir, parentCycleID)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func captureExecuteGitSnapshot(ctx context.Context, repo git.GitRepo, repoRoot, workdir, priorCycleBase string) (git.PhaseSnapshot, error) {
	return git.CaptureExecuteGitSnapshot(ctx, repo, repoRoot, workdir, priorCycleBase)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mergeRunnerDetailsWithGit(baseDetails []byte, snap git.PhaseSnapshot, commitCount int) []byte {
	return git.MergeRunnerDetailsWithGit(baseDetails, snap, commitCount)
}
