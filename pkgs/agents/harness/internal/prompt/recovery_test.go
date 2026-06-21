package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestComposeRecoveryDelta_verifyImplementation(t *testing.T) {
	t.Parallel()
	delta := ComposeRecoveryDelta(RecoveryContext{
		Kind:       RecoveryVerifyImplementation,
		Phase:      domain.PhaseExecute,
		AttemptSeq: 2,
		ReportPath: "/tmp/hamix/cycle-1/criteria-report.json",
		FailedCriteria: []CriterionFailure{{
			ID:        "criterion-a",
			Reasoning: "missing handler",
			Verifier:  "verify_agent",
		}},
		LockedCriteria: []string{"criterion-b"},
	})
	if !strings.Contains(delta, "## Continuation (Hamix attempt 2)") {
		t.Fatalf("missing continuation header: %q", delta)
	}
	if !strings.Contains(delta, "**[criterion-a]**") {
		t.Fatalf("missing structured failure: %q", delta)
	}
	if !strings.Contains(delta, "criterion-b") {
		t.Fatalf("missing locked criteria: %q", delta)
	}
}

func TestComposeRecoveryDelta_criteriaReportInvalid(t *testing.T) {
	t.Parallel()
	delta := ComposeRecoveryDelta(RecoveryContext{
		Kind:           RecoveryCriteriaReportInvalid,
		Phase:          domain.PhaseExecute,
		AttemptSeq:     3,
		ReportPath:     "/tmp/report.json",
		ReportParseErr: "criteria report invalid: duplicate criterion id a",
		ExpectedIDs:    []string{"a", "c"},
		LockedCriteria: []string{"b"},
	})
	if !strings.Contains(delta, "Parse error:") {
		t.Fatalf("missing parse error: %q", delta)
	}
	if !strings.Contains(delta, "Expected criterion IDs: a, c") {
		t.Fatalf("missing expected ids: %q", delta)
	}
}

func TestComposeRecoveryDelta_verifyInfra(t *testing.T) {
	t.Parallel()
	delta := ComposeRecoveryDelta(RecoveryContext{
		Kind:          RecoveryVerifyInfra,
		Phase:         domain.PhaseVerify,
		AttemptSeq:    1,
		VerifyAttempt: 1,
		CommandEvidenceDelta: []CommandEvidenceLine{{
			CriterionID: "lint",
			Command:     "npm test",
			ExitCode:    1,
			Preview:     "FAIL",
		}},
	})
	if !strings.Contains(delta, "### New command evidence") {
		t.Fatalf("missing command evidence section: %q", delta)
	}
	if !strings.Contains(delta, "npm test") {
		t.Fatalf("missing command: %q", delta)
	}
}

func TestComposeRecoveryDelta_goldenFiles(t *testing.T) {
	cases := map[string]RecoveryContext{
		"verify_implementation_fail": {
			Kind:       RecoveryVerifyImplementation,
			Phase:      domain.PhaseExecute,
			AttemptSeq: 2,
			ReportPath: "/tmp/hamix/cycle-1/criteria-report.json",
			FailedCriteria: []CriterionFailure{{
				ID: "criterion-a", Reasoning: "missing handler", Verifier: "verify_agent",
			}},
			LockedCriteria: []string{"criterion-b"},
		},
		"criteria_report_invalid": {
			Kind:           RecoveryCriteriaReportInvalid,
			Phase:          domain.PhaseExecute,
			AttemptSeq:     3,
			ReportPath:     "/tmp/report.json",
			ReportParseErr: "criteria report invalid: duplicate criterion id a",
			ExpectedIDs:    []string{"a", "c"},
			LockedCriteria: []string{"b"},
		},
		"criteria_report_missing": {
			Kind:        RecoveryCriteriaReportMissing,
			Phase:       domain.PhaseExecute,
			AttemptSeq:  2,
			ReportPath:  "/tmp/report.json",
			ExpectedIDs: []string{"a"},
		},
		"process_restart": {
			Kind:             RecoveryProcessRestart,
			Phase:            domain.PhaseExecute,
			AttemptSeq:       1,
			InterruptedPhase: domain.PhaseExecute,
			FailureReason:    "shutdown",
		},
		"operator_retry_resume": {
			Kind:           RecoveryOperatorRetryResume,
			Phase:          domain.PhaseExecute,
			AttemptSeq:     2,
			FailureClass:   "verify",
			FailureReason:  "verification_failed",
			ScopeFiles:     []string{"src/foo.go"},
			LockedCriteria: []string{"criterion-b"},
			ReportPath:     "/tmp/report.json",
		},
		"verify_infra_retry": {
			Kind:          RecoveryVerifyInfra,
			Phase:         domain.PhaseVerify,
			AttemptSeq:    1,
			VerifyAttempt: 1,
			CommandEvidenceDelta: []CommandEvidenceLine{{
				CriterionID: "lint", Command: "npm test", ExitCode: 1, Preview: "FAIL",
			}},
		},
		"verify_feedback_carry": {
			Kind:                RecoveryVerifyFeedback,
			Phase:               domain.PhaseVerify,
			AttemptSeq:          1,
			VerifyAttempt:       2,
			PriorVerifyFeedback: "criterion-a: still failing",
			LockedCriteria:      []string{"criterion-b"},
		},
	}
	for name, ctx := range cases {
		name, ctx := name, ctx
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := ComposeRecoveryDelta(ctx)
			path := filepath.Join("testdata", "recovery", name+".txt")
			if os.Getenv("UPDATE_RECOVERY_GOLDEN") != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with UPDATE_RECOVERY_GOLDEN=1)", path, err)
			}
			if strings.TrimSpace(got) != strings.TrimSpace(string(want)) {
				t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}
