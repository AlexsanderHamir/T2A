package harness

import (
	"context"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/orchestration"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func verdictsToClassifyInput(verdicts []criterionVerdict) []orchestration.ClassifyVerdict {
	out := make([]orchestration.ClassifyVerdict, len(verdicts))
	for i, v := range verdicts {
		out[i] = orchestration.ClassifyVerdict{Passed: v.Passed, Verifier: v.Verifier}
	}
	return out
}

func (h *Harness) anchorPostExecuteState(
	ctx context.Context,
	state *processState,
	execPhaseSeq int64,
	snap git.PhaseSnapshot,
	ingestAttempted bool,
	ingestOutcome executeCommitIngestOutcome,
	ingestErr error,
) {
	state.executeReachedVerify = true
	state.lastCommitIngestOK = commitIngestOK(snap, ingestAttempted, ingestOutcome, ingestErr)
	head, ok, err := h.resolveCurrentHeadSHA(ctx, snap)
	if err != nil {
		slog.Warn("agent harness post-execute head anchor failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.Harness.anchorPostExecuteState.head",
			"cycle_id", state.cycleID, "err", err)
		return
	}
	if ok {
		state.postExecuteHeadSHA = head
	}
	state.lastCompletedExecutePhaseSeq = execPhaseSeq
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func commitIngestOK(
	snap git.PhaseSnapshot,
	ingestAttempted bool,
	ingestOutcome executeCommitIngestOutcome,
	ingestErr error,
) bool {
	if snap.Skipped {
		return true
	}
	if !ingestAttempted {
		return true
	}
	if ingestErr != nil {
		return false
	}
	return ingestOutcome.FailReason == ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) resolveCurrentHeadSHA(ctx context.Context, snap git.PhaseSnapshot) (head string, ok bool, err error) {
	if snap.Skipped {
		return "", false, nil
	}
	workdir := strings.TrimSpace(h.opts.WorkingDir)
	if workdir == "" {
		return "", false, nil
	}
	repo := h.gitSvc().Repo()
	if repo == nil {
		return "", false, nil
	}
	out, err := repo.Run(ctx, workdir, "rev-parse", "HEAD")
	if err != nil {
		if git.IsNotAGitRepoErr(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) gatherRetryClassifyInput(
	ctx context.Context,
	cycle *domain.TaskCycle,
	state *processState,
	verdicts []criterionVerdict,
	verifyErr error,
) orchestration.ClassifyInput {
	reportValid := true
	h.probeCriteriaReport(state, cycle.ID)
	if state.reportParseErr != "" {
		reportValid = false
	}
	headMatches := true
	if state.postExecuteHeadSHA != "" {
		current, ok, err := h.resolveCurrentHeadSHA(ctx, state.gitSnap)
		if err != nil {
			headMatches = false
		} else if ok {
			headMatches = strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(state.postExecuteHeadSHA))
		}
	}
	pipelineFailed := verifyErr != nil
	failureClass := orchestration.ClassifyFailureClass(verdictsToClassifyInput(verdicts), pipelineFailed)
	return orchestration.ClassifyInput{
		FailureClass:         failureClass,
		CriteriaReportValid:  reportValid,
		GitHeadMatchesAnchor: headMatches,
		CommitIngestOK:       state.lastCommitIngestOK,
		ExecuteReachedVerify: state.executeReachedVerify,
	}
}
