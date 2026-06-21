package resume

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// Entry selects which cycle-loop branch resumes after interruption or retry.
type Entry int

const (
	EntryExecute Entry = iota
	EntryVerifyOnly
	EntryAfterExecuteSuccess
)

// FailureClass categorizes the parent attempt failure for continuation prompts.
type FailureClass string

const (
	FailureClassRunner         FailureClass = "runner"
	FailureClassExecuteGate    FailureClass = "executeGate"
	FailureClassVerify         FailureClass = "verify"
	FailureClassInfrastructure FailureClass = "infrastructure"
	FailureClassOperator       FailureClass = "operator"
)

// CriterionVerdict records a locked pass from a prior verify attempt.
type CriterionVerdict struct {
	ID        string
	Passed    bool
	Evidence  string
	Verifier  domain.VerifierKind
	Reasoning string
}

// ContinuationBundle rehydrates cross-cycle resume context from a parent attempt.
type ContinuationBundle struct {
	Entry                  Entry
	LineageAttempt         int64
	ParentCycleID          string
	FailureClass           FailureClass
	FailureReason          string
	FailurePhase           domain.Phase
	ScopeFiles             []string
	Commits                []domain.TaskCycleCommit
	CriteriaEvidence       []domain.TaskCycleCriteriaReport
	PreviouslyPassed       map[string]CriterionVerdict
	VerifyFeedback         string
	ExecuteFeedback        string
	CriteriaReportProbeErr string
	RunnerFeedback         string
	GitDiagnostics         string
	Warnings               []string
	Sufficient             bool
}

// Checkpoint is the in-cycle resume state reconstructed from the phase ledger.
type Checkpoint struct {
	Entry            Entry
	PreviouslyPassed map[string]CriterionVerdict
	VerifyAttempt    int
	VerifyFeedback   string
	KnownCommits     []domain.TaskCycleCommit
	Continuation     *ContinuationBundle
}
