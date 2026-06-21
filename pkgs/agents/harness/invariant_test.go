package harness

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/orchestration"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// Invariant: verify retry decisions never return retry after the budget is exhausted.
// Locks orchestration contract used by runCycleLoopVerify (ADR-0018).
func TestInvariant_verifyRetryNeverExceedsBudget(t *testing.T) {
	t.Parallel()
	max := 3
	for attempt := 0; attempt <= max+1; attempt++ {
		effects := orchestration.DecideVerifyRetry(attempt, max, orchestration.VerifyResultFailRetryable)
		if attempt < max && !effects.RetryLoop {
			t.Fatalf("attempt %d: want retry", attempt)
		}
		if attempt >= max && (effects.RetryLoop || !effects.TerminalFailure) {
			t.Fatalf("attempt %d: want terminal failure without retry, got %+v", attempt, effects)
		}
	}
}

// Invariant: tamper is always terminal — never retried through the execute↔verify loop.
func TestInvariant_tamperNeverRetries(t *testing.T) {
	t.Parallel()
	for attempt := 0; attempt < 5; attempt++ {
		effects := orchestration.DecideVerifyRetry(attempt, 10, orchestration.VerifyResultFailTampered)
		if effects.RetryLoop || !effects.TerminalFailure || !effects.Tampered {
			t.Fatalf("attempt %d: tamper must be terminal, got %+v", attempt, effects)
		}
	}
}

// Invariant: execute terminal effects never also continue to verify.
func TestInvariant_executeNeverContinuesAfterTerminalFailure(t *testing.T) {
	t.Parallel()
	terminalInputs := []orchestration.ExecutePostRunInput{
		{RunnerOutcome: orchestration.ExecuteRunnerOutcomeTimeout},
		{RunnerOutcome: orchestration.ExecuteRunnerOutcomeOK, OperatorCancelled: true},
		{
			RunnerOutcome: orchestration.ExecuteRunnerOutcomeOK,
			CommitIngest: orchestration.ExecuteCommitIngestSummary{
				IngestAttempted: true,
				IngestErr:       true,
			},
		},
	}
	for i, in := range terminalInputs {
		e := orchestration.DecideExecutePostRun(in)
		if e.TerminateFailed && e.ContinueToVerify {
			t.Fatalf("input %d: terminal and continue both set: %+v", i, e)
		}
		if e.TerminateFailed && e.StopLoop {
			t.Fatalf("input %d: terminal and stop both set: %+v", i, e)
		}
	}
}

// Invariant: terminal cycle statuses are mutually exclusive with an open running phase
// in the domain model — harness termination paths must land on a terminal status.
func TestInvariant_terminalCycleStatusesAreTerminal(t *testing.T) {
	t.Parallel()
	for _, st := range []domain.CycleStatus{
		domain.CycleStatusSucceeded,
		domain.CycleStatusFailed,
		domain.CycleStatusAborted,
	} {
		if !domain.TerminalCycleStatus(st) {
			t.Fatalf("%q should be terminal", st)
		}
	}
	if domain.TerminalCycleStatus(domain.CycleStatusRunning) {
		t.Fatal("running must not be terminal")
	}
}
