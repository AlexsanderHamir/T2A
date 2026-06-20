package harness

import (
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume"
)

// ContinuationBundle rehydrates cross-cycle resume context from a parent attempt.
type ContinuationBundle = resume.ContinuationBundle

type resumeCheckpoint = resume.Checkpoint
type resumeEntry = resume.Entry

const (
	resumeEntryExecute             = resume.EntryExecute
	resumeEntryVerifyOnly          = resume.EntryVerifyOnly
	resumeEntryAfterExecuteSuccess = resume.EntryAfterExecuteSuccess
)

type failureClass = resume.FailureClass

const (
	failureClassRunner         = resume.FailureClassRunner
	failureClassExecuteGate    = resume.FailureClassExecuteGate
	failureClassVerify         = resume.FailureClassVerify
	failureClassInfrastructure = resume.FailureClassInfrastructure
	failureClassOperator       = resume.FailureClassOperator
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) resumeSvc() *resume.Service {
	if h.resume == nil {
		h.resume = resume.NewService(h.store, resume.Options{
			WorkingDir: h.opts.WorkingDir,
			GitRepo:    h.gitSvc().Repo(),
		})
	}
	return h.resume
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func harnessVerdictsFromResume(m map[string]resume.CriterionVerdict) map[string]criterionVerdict {
	if len(m) == 0 {
		return map[string]criterionVerdict{}
	}
	out := make(map[string]criterionVerdict, len(m))
	for id, v := range m {
		key := v.ID
		if key == "" {
			key = id
		}
		out[key] = criterionVerdict{
			ID:        key,
			Passed:    v.Passed,
			Evidence:  v.Evidence,
			Verifier:  v.Verifier,
			Reasoning: v.Reasoning,
		}
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func reasonRemediation(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	return "Prior attempt failed: " + reason
}
