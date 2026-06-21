package orchestration

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

// TerminationReason is a stable cycle terminate_reason string persisted to the store.
type TerminationReason string

const (
	ReasonVerifyTampered            TerminationReason = "verify_tampered"
	ReasonRunnerTimeout             TerminationReason = "runner_timeout"
	ReasonRunnerNonZeroExit         TerminationReason = "runner_non_zero_exit"
	ReasonRunnerInvalidOutput       TerminationReason = "runner_invalid_output"
	ReasonRunnerError               TerminationReason = "runner_error"
	ReasonRunnerStale               TerminationReason = "runner_stale"
	ReasonExecuteInvalidCommit      TerminationReason = "execute_invalid_commit"
	ReasonCancelledByOperator       TerminationReason = "cancelled_by_operator"
	ReasonChecklistCompletionFailed TerminationReason = "checklist_completion_failed"
)

// VerifyEffects lists side effects the harness root applies after DecideVerifyRetry.
type VerifyEffects struct {
	RetryLoop       bool
	SkipNextExecute bool
	TerminalFailure bool
	Tampered        bool
}

// ExecuteEffects lists side effects the harness root applies after DecideExecutePostRun.
type ExecuteEffects struct {
	ContinueToVerify bool
	StopLoop         bool
	TerminateFailed  bool
	TransitionTask   domain.Status
	Reason           TerminationReason
	ResultSummary    string
}

// FinalizeEffects lists side effects after DecideFinalizeSuccess.
type FinalizeEffects struct {
	CycleStatus domain.CycleStatus
	TaskStatus  domain.Status
	Reason      TerminationReason
}
