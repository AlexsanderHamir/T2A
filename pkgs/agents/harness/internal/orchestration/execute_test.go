package orchestration

import (
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestDecideExecutePostRun_runnerOK(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome: ExecuteRunnerOutcomeOK,
	})
	if !e.ContinueToVerify || e.TerminateFailed || e.StopLoop {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideExecutePostRun_contextCancelled(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{ContextCancelled: true})
	if !e.StopLoop || e.ContinueToVerify || e.TerminateFailed {
		t.Fatalf("unexpected effects: %+v", e)
	}
}

func TestDecideExecutePostRun_runnerErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		outcome ExecuteRunnerOutcome
		reason  TerminationReason
	}{
		{ExecuteRunnerOutcomeTimeout, ReasonRunnerTimeout},
		{ExecuteRunnerOutcomeNonZeroExit, ReasonRunnerNonZeroExit},
		{ExecuteRunnerOutcomeInvalidOutput, ReasonRunnerInvalidOutput},
		{ExecuteRunnerOutcomeError, ReasonRunnerError},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			t.Parallel()
			e := DecideExecutePostRun(ExecutePostRunInput{RunnerOutcome: tt.outcome})
			if !e.TerminateFailed || e.TransitionTask != domain.StatusFailed || e.Reason != tt.reason {
				t.Fatalf("got %+v", e)
			}
		})
	}
}

func TestDecideExecutePostRun_operatorCancelAfterRunnerOK(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome:     ExecuteRunnerOutcomeOK,
		OperatorCancelled: true,
	})
	if !e.TerminateFailed || e.Reason != ReasonCancelledByOperator || e.ResultSummary != "cancelled by operator" {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_operatorCancelOverlaysRunnerFailure(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome:     ExecuteRunnerOutcomeTimeout,
		OperatorCancelled: true,
	})
	if e.Reason != ReasonCancelledByOperator || e.ResultSummary != "cancelled by operator" {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_commitIngestErr(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome: ExecuteRunnerOutcomeOK,
		CommitIngest: ExecuteCommitIngestSummary{
			IngestAttempted: true,
			IngestErr:       true,
		},
	})
	if !e.TerminateFailed || e.Reason != ReasonExecuteInvalidCommit {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_commitIngestFailReasonOnlyIO(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome: ExecuteRunnerOutcomeOK,
		CommitIngest: ExecuteCommitIngestSummary{
			IngestAttempted: true,
			FailReason:      "execute_invalid_commit",
		},
	})
	if !e.TerminateFailed || string(e.Reason) != "execute_invalid_commit" {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_emptyClaimsContinue(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome: ExecuteRunnerOutcomeOK,
		CommitIngest: ExecuteCommitIngestSummary{
			IngestAttempted: true,
		},
	})
	if !e.ContinueToVerify || e.TerminateFailed {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_gitSkippedSkipsIngest(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		RunnerOutcome: ExecuteRunnerOutcomeOK,
		CommitIngest: ExecuteCommitIngestSummary{
			GitSnapshotSkipped: true,
		},
	})
	if !e.ContinueToVerify {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_evidenceRecoveryContinue(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		EvidenceRecovery: true,
		CommitIngest: ExecuteCommitIngestSummary{
			IngestAttempted: true,
		},
	})
	if !e.ContinueToVerify || e.TerminateFailed {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideExecutePostRun_evidenceRecoveryContinueWithEmptyClaims(t *testing.T) {
	t.Parallel()
	e := DecideExecutePostRun(ExecutePostRunInput{
		EvidenceRecovery: true,
		CommitIngest: ExecuteCommitIngestSummary{
			IngestAttempted: true,
		},
	})
	if !e.ContinueToVerify || e.TerminateFailed {
		t.Fatalf("got %+v", e)
	}
}

func TestDecideVerifyDisabledLegacy(t *testing.T) {
	t.Parallel()
	if e := DecideVerifyDisabledLegacy(nil); e.TerminalFailure {
		t.Fatalf("nil err: %+v", e)
	}
	if e := DecideVerifyDisabledLegacy(errSentinel{}); !e.TerminalFailure {
		t.Fatalf("want terminal: %+v", e)
	}
}

func TestDecideFinalizeSuccess(t *testing.T) {
	t.Parallel()
	ok := DecideFinalizeSuccess(nil)
	if ok.CycleStatus != domain.CycleStatusSucceeded || ok.TaskStatus != domain.StatusDone {
		t.Fatalf("got %+v", ok)
	}
	fail := DecideFinalizeSuccess(errSentinel{})
	if fail.CycleStatus != domain.CycleStatusFailed || fail.Reason != ReasonChecklistCompletionFailed {
		t.Fatalf("got %+v", fail)
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel" }
