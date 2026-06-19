package verify

import (
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const logCmd = "taskapi"

const failedReasonPrefix = "verification_failed"

// FailedReasonPrefix is the stable terminate_reason prefix for verification failures.
const FailedReasonPrefix = failedReasonPrefix

// Snapshot captures verify settings and checklist criteria for one task run.
type Snapshot struct {
	Enabled                     bool
	MaxRetries                  int
	VerifyCommandTimeoutSeconds int
	Criteria                    []store.ChecklistVerifyItem
	VerifyRunner                runner.Runner
	VerifyModel                 string
}

// Verdict is the harness-internal outcome for one criterion after verify work.
type Verdict struct {
	ID        string
	Passed    bool
	Evidence  string
	Verifier  domain.VerifierKind
	Reasoning string
}

// TamperedError is returned when post-verify integrity detects unauthorized
// working-tree changes. Terminal for the cycle — callers use errors.As and
// map to verify_tampered terminate reason.
type TamperedError struct {
	Reason string
}

func (e *TamperedError) Error() string {
	if e == nil {
		return ""
	}
	return "verify_tampered: " + e.Reason
}
