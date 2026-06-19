package harness

import (
	"context"
	"errors"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/orchestration"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func mapRunnerOutcome(err error) orchestration.ExecuteRunnerOutcome {
	if err == nil {
		return orchestration.ExecuteRunnerOutcomeOK
	}
	switch {
	case errors.Is(err, runner.ErrTimeout):
		return orchestration.ExecuteRunnerOutcomeTimeout
	case errors.Is(err, runner.ErrNonZeroExit):
		return orchestration.ExecuteRunnerOutcomeNonZeroExit
	case errors.Is(err, runner.ErrInvalidOutput):
		return orchestration.ExecuteRunnerOutcomeInvalidOutput
	default:
		return orchestration.ExecuteRunnerOutcomeError
	}
}

func buildExecutePostRunInput(
	parentCtx context.Context,
	runErr error,
	operatorCancelled bool,
	snap git.PhaseSnapshot,
	ingestAttempted bool,
	ingestOutcome executeCommitIngestOutcome,
	ingestErr error,
) orchestration.ExecutePostRunInput {
	in := orchestration.ExecutePostRunInput{
		RunnerOutcome:     mapRunnerOutcome(runErr),
		OperatorCancelled: operatorCancelled,
		ContextCancelled:  parentCtx.Err() != nil,
		CommitIngest: orchestration.ExecuteCommitIngestSummary{
			GitSnapshotSkipped: snap.Skipped,
		},
	}
	if runErr == nil && !operatorCancelled && !snap.Skipped {
		in.CommitIngest.IngestAttempted = ingestAttempted
		if ingestAttempted {
			in.CommitIngest.IngestErr = ingestErr != nil
			in.CommitIngest.FailReason = ingestOutcome.FailReason
		}
	}
	return in
}

func executePhaseStatusFromEffects(effects orchestration.ExecuteEffects) domain.PhaseStatus {
	if effects.ContinueToVerify {
		return domain.PhaseStatusSucceeded
	}
	return domain.PhaseStatusFailed
}

func overlayOperatorCancelOnResult(result runner.Result, operatorCancelled bool, effects orchestration.ExecuteEffects) runner.Result {
	if !operatorCancelled {
		return result
	}
	if result.Summary == "" || strings.HasPrefix(result.Summary, "cursor: timeout") {
		summary := effects.ResultSummary
		if summary == "" {
			summary = "cancelled by operator"
		}
		result.Summary = summary
	}
	return result
}
