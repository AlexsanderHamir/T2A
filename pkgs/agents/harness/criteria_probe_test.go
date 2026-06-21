package harness

import (
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestSelectRecoveryKind_criteriaReportInvalidFromState(t *testing.T) {
	t.Parallel()
	h := &Harness{}
	state := &processState{
		reportParseErr: "criteria report invalid: unknown field function",
	}
	kind := h.selectRecoveryKind(domain.PhaseExecute, state, cycleLoopOpts{}, domain.RetryFresh)
	if kind != prompt.RecoveryCriteriaReportInvalid {
		t.Fatalf("kind=%q want %q", kind, prompt.RecoveryCriteriaReportInvalid)
	}
}

func TestSelectRecoveryKind_operatorRetryDefersToCriteriaProbeErr(t *testing.T) {
	t.Parallel()
	h := &Harness{}
	state := &processState{
		reportParseErr: "criteria report missing",
	}
	opts := cycleLoopOpts{continuation: &ContinuationBundle{ParentCycleID: "parent-1"}}
	kind := h.selectRecoveryKind(domain.PhaseExecute, state, opts, domain.RetryResume)
	if kind != prompt.RecoveryCriteriaReportMissing {
		t.Fatalf("kind=%q want criteria_report_missing", kind)
	}
}
