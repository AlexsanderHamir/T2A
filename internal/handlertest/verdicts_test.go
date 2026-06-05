package handlertest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// TestHandler_GetCycleVerdicts_returnsBothReports pins the
// /tasks/{id}/cycles/{cycleId}/verdicts contract end-to-end via
// httptest:
//   - both criteria_reports and verify_reports come back in the
//     envelope, ordered by (attempt_seq ASC, criterion_id ASC)
//   - the verifier_kind enum survives JSON serialization
//   - a freshly-created cycle (no upserts) returns 200 with empty
//     arrays — never 404 / 500 — so the SPA can render "no verdicts
//     captured" without special-casing legacy cycles.
func TestHandler_GetCycleVerdicts_returnsBothReports(t *testing.T) {
	t.Parallel()
	srv, st := NewServerWithStore(t)
	defer srv.Close()

	ctx := context.Background()

	tsk, err := st.Create(ctx, store.CreateTaskInput{Priority: domain.PriorityMedium, Title: "verdict-test"}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	c1, err := st.AddChecklistItem(ctx, tsk.ID, "criterion one", "", domain.ActorUser)
	if err != nil {
		t.Fatalf("add c1: %v", err)
	}
	c2, err := st.AddChecklistItem(ctx, tsk.ID, "criterion two", "", domain.ActorUser)
	if err != nil {
		t.Fatalf("add c2: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}

	// Empty case first — no rows persisted yet — must be 200 with
	// empty arrays, not 404. This is the pre-PR2-cycle contract.
	resp := getVerdicts(t, srv.URL, tsk.ID, cycle.ID)
	if len(resp.CriteriaReports) != 0 || len(resp.VerifyReports) != 0 {
		t.Fatalf("empty cycle returned non-empty arrays: %+v", resp)
	}
	if resp.TaskID != tsk.ID || resp.CycleID != cycle.ID {
		t.Fatalf("envelope ids mismatch: %+v", resp)
	}

	// Mirror what the worker would persist: c1 passes via verify_agent
	// on attempt 1; c2 is claimed-but-not-verified on attempt 1.
	if err := st.UpsertCriteriaReports(ctx, cycle.ID, 1, []store.CriteriaReportEntry{
		{CriterionID: c1.ID, ClaimedDone: true, Evidence: "ev-c1"},
		{CriterionID: c2.ID, ClaimedDone: true, Evidence: "ev-c2"},
	}); err != nil {
		t.Fatalf("upsert criteria: %v", err)
	}
	if err := st.UpsertVerifyReports(ctx, cycle.ID, 1, []store.VerifyReportEntry{
		{
			CriterionID:  c1.ID,
			Verified:     true,
			VerifierKind: domain.VerifierVerifyAgent,
			Reasoning:    "criterion one passes",
		},
		{
			CriterionID:  c2.ID,
			Verified:     false,
			VerifierKind: domain.VerifierVerifyAgent,
			Reasoning:    "criterion two failed",
		},
	}); err != nil {
		t.Fatalf("upsert verify: %v", err)
	}

	resp = getVerdicts(t, srv.URL, tsk.ID, cycle.ID)
	if len(resp.CriteriaReports) != 2 {
		t.Fatalf("criteria_reports len = %d, want 2", len(resp.CriteriaReports))
	}
	if len(resp.VerifyReports) != 2 {
		t.Fatalf("verify_reports len = %d, want 2", len(resp.VerifyReports))
	}
	for i, row := range resp.VerifyReports {
		if row.VerifierKind != string(domain.VerifierVerifyAgent) {
			t.Errorf("verify_reports[%d].verifier_kind = %q, want %q", i, row.VerifierKind, domain.VerifierVerifyAgent)
		}
	}
	if resp.VerifyReports[0].Verified == resp.VerifyReports[1].Verified {
		t.Errorf("expected one passing and one failing verdict, got both %v", resp.VerifyReports[0].Verified)
	}
}

// verdictsResponse mirrors the handler's response DTO for read-only
// JSON parsing. We don't import the unexported handler type because
// tests live in handlertest package, which is intentionally limited
// to exported API surface.
type verdictsResponse struct {
	TaskID          string `json:"task_id"`
	CycleID         string `json:"cycle_id"`
	CriteriaReports []struct {
		ID          string `json:"id"`
		CycleID     string `json:"cycle_id"`
		AttemptSeq  int64  `json:"attempt_seq"`
		CriterionID string `json:"criterion_id"`
		ClaimedDone bool   `json:"claimed_done"`
		Evidence    string `json:"evidence"`
		WrittenAt   string `json:"written_at"`
	} `json:"criteria_reports"`
	VerifyReports []struct {
		ID           string `json:"id"`
		CycleID      string `json:"cycle_id"`
		AttemptSeq   int64  `json:"attempt_seq"`
		CriterionID  string `json:"criterion_id"`
		Verified     bool   `json:"verified"`
		VerifierKind string `json:"verifier_kind"`
		Reasoning    string `json:"reasoning"`
		WrittenAt    string `json:"written_at"`
	} `json:"verify_reports"`
}

func getVerdicts(t *testing.T, base, taskID, cycleID string) verdictsResponse {
	t.Helper()
	url := fmt.Sprintf("%s/tasks/%s/cycles/%s/verdicts", base, taskID, cycleID)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	var out verdictsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
