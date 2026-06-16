package harness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestFormatVerifyPhaseSummary_success(t *testing.T) {
	t.Parallel()
	criteria := []store.ChecklistVerifyItem{
		{ID: "c1", Text: "Ship tests"},
		{ID: "c2", Text: "Update docs"},
	}
	verdicts := []criterionVerdict{
		{id: "c1", passed: true, verifier: domain.VerifierVerifyAgent},
		{id: "c2", passed: true, verifier: domain.VerifierVerifyAgent},
	}
	got := formatVerifyPhaseSummary(criteria, verdicts, true)
	if got != "All 2 criteria verified" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatVerifyPhaseSummary_failureListsReasoning(t *testing.T) {
	t.Parallel()
	criteria := []store.ChecklistVerifyItem{
		{ID: "c1", Text: "Each branch has a test"},
		{ID: "c2", Text: "Docs updated"},
	}
	verdicts := []criterionVerdict{
		{id: "c1", passed: false, reasoning: "No test for limit=201"},
		{id: "c2", passed: true},
	}
	got := formatVerifyPhaseSummary(criteria, verdicts, false)
	if !strings.HasPrefix(got, "1 of 2 criteria failed") {
		t.Fatalf("headline: got %q", got)
	}
	if !strings.Contains(got, "Each branch has a test") {
		t.Fatalf("criterion text missing: %q", got)
	}
	if !strings.Contains(got, "No test for limit=201") {
		t.Fatalf("reasoning missing: %q", got)
	}
}

func TestEncodeVerifyPhaseDetails_includesStructuredSnapshot(t *testing.T) {
	t.Parallel()
	criteria := []store.ChecklistVerifyItem{
		{ID: "c1", Text: "Criterion A"},
	}
	verdicts := []criterionVerdict{
		{
			id:        "c1",
			passed:    false,
			verifier:  domain.VerifierVerifyAgent,
			reasoning: "Missing coverage",
		},
	}
	raw := encodeVerifyPhaseDetails(2, criteria, verdicts)
	var got verifyPhaseDetailsPayload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Verification.AttemptSeq != 2 {
		t.Fatalf("attempt_seq = %d", got.Verification.AttemptSeq)
	}
	if got.Verification.FailedCount != 1 || got.Verification.PassedCount != 0 {
		t.Fatalf("counts: passed=%d failed=%d", got.Verification.PassedCount, got.Verification.FailedCount)
	}
	if len(got.Verification.Criteria) != 1 {
		t.Fatalf("criteria len = %d", len(got.Verification.Criteria))
	}
	row := got.Verification.Criteria[0]
	if row.CriterionID != "c1" || row.Text != "Criterion A" || row.Verified {
		t.Fatalf("row: %+v", row)
	}
	if row.VerifierKind != string(domain.VerifierVerifyAgent) {
		t.Fatalf("verifier_kind = %q", row.VerifierKind)
	}
	if row.Reasoning != "Missing coverage" {
		t.Fatalf("reasoning = %q", row.Reasoning)
	}
}
