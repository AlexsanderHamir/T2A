package orchestration

import (
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const orchestrationLogCmd = "taskapi"

// DecideExecutePostRun maps execute post-run facts to effects. The harness root
// applies store writes; this function is pure policy only.
func DecideExecutePostRun(in ExecutePostRunInput) ExecuteEffects {
	if in.ContextCancelled {
		return ExecuteEffects{StopLoop: true}
	}

	if in.EvidenceRecovery {
		return decideExecuteAfterEvidenceRecovery(in)
	}

	effects := executeEffectsFromRunner(in.RunnerOutcome)
	if effects.TerminateFailed {
		effects = overlayOperatorCancel(in.OperatorCancelled, effects)
		return effects
	}

	if in.OperatorCancelled {
		return ExecuteEffects{
			TerminateFailed: true,
			TransitionTask:  domain.StatusFailed,
			Reason:          ReasonCancelledByOperator,
			ResultSummary:   "cancelled by operator",
		}
	}

	if in.CommitIngest.GitSnapshotSkipped || !in.CommitIngest.IngestAttempted {
		return ExecuteEffects{ContinueToVerify: true}
	}

	if in.CommitIngest.IngestErr {
		return ExecuteEffects{
			TerminateFailed: true,
			TransitionTask:  domain.StatusFailed,
			Reason:          ReasonExecuteInvalidCommit,
			ResultSummary:   string(ReasonExecuteInvalidCommit),
		}
	}
	if in.CommitIngest.FailReason != "" {
		return ExecuteEffects{
			TerminateFailed: true,
			TransitionTask:  domain.StatusFailed,
			Reason:          TerminationReason(in.CommitIngest.FailReason),
			ResultSummary:   in.CommitIngest.FailReason,
		}
	}

	return ExecuteEffects{ContinueToVerify: true}
}

func decideExecuteAfterEvidenceRecovery(in ExecutePostRunInput) ExecuteEffects {
	slog.Debug("trace", "cmd", orchestrationLogCmd, "operation", "agent.harness.orchestration.decideExecuteAfterEvidenceRecovery",
		"operator_cancelled", in.OperatorCancelled, "evidence_recovery", in.EvidenceRecovery)
	if in.OperatorCancelled {
		return ExecuteEffects{
			TerminateFailed: true,
			TransitionTask:  domain.StatusFailed,
			Reason:          ReasonCancelledByOperator,
			ResultSummary:   "cancelled by operator",
		}
	}

	if in.CommitIngest.GitSnapshotSkipped || !in.CommitIngest.IngestAttempted {
		return ExecuteEffects{ContinueToVerify: true}
	}
	if in.CommitIngest.IngestErr {
		return ExecuteEffects{
			TerminateFailed: true,
			TransitionTask:  domain.StatusFailed,
			Reason:          ReasonExecuteInvalidCommit,
			ResultSummary:   string(ReasonExecuteInvalidCommit),
		}
	}
	if in.CommitIngest.FailReason != "" {
		reason := TerminationReason(in.CommitIngest.FailReason)
		if reason == "" {
			reason = ReasonRunnerStale
		}
		return ExecuteEffects{
			TerminateFailed: true,
			TransitionTask:  domain.StatusFailed,
			Reason:          reason,
			ResultSummary:   in.CommitIngest.FailReason,
		}
	}
	return ExecuteEffects{ContinueToVerify: true}
}

func executeEffectsFromRunner(outcome ExecuteRunnerOutcome) ExecuteEffects {
	switch outcome {
	case ExecuteRunnerOutcomeOK:
		return ExecuteEffects{ContinueToVerify: true}
	case ExecuteRunnerOutcomeTimeout:
		return terminalExecute(domain.StatusFailed, ReasonRunnerTimeout)
	case ExecuteRunnerOutcomeNonZeroExit:
		return terminalExecute(domain.StatusFailed, ReasonRunnerNonZeroExit)
	case ExecuteRunnerOutcomeInvalidOutput:
		return terminalExecute(domain.StatusFailed, ReasonRunnerInvalidOutput)
	default:
		return terminalExecute(domain.StatusFailed, ReasonRunnerError)
	}
}

func terminalExecute(taskStatus domain.Status, reason TerminationReason) ExecuteEffects {
	return ExecuteEffects{
		TerminateFailed: true,
		TransitionTask:  taskStatus,
		Reason:          reason,
	}
}

func overlayOperatorCancel(operatorCancelled bool, effects ExecuteEffects) ExecuteEffects {
	if !operatorCancelled {
		return effects
	}
	effects.Reason = ReasonCancelledByOperator
	if effects.ResultSummary == "" {
		effects.ResultSummary = "cancelled by operator"
	}
	return effects
}
