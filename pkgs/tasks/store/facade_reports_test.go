package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// seedCycleWithCriterion creates a task + one checklist criterion + one
// running cycle so verdict-row tests can exercise the
// (cycle_id, attempt_seq, criterion_id) FK chain end to end.
func seedCycleWithCriterion(t *testing.T, s *Store, ctx context.Context) (cycleID, criterionID string) {
	t.Helper()
	tsk := mustCreateTask(t, s, ctx)
	it, err := s.AddChecklistItem(ctx, tsk.ID, "criterion", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}
	cycle, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	return cycle.ID, it.ID
}

// TestStore_UpsertCriteriaReports_round_trip pins the happy path:
// one bulk insert lands rows, the list query returns them in
// (attempt_seq, criterion_id) order, and a re-upsert with new
// values updates the existing row rather than inserting a duplicate.
func TestStore_UpsertCriteriaReports_round_trip(t *testing.T) {
	s, ctx := newCycleStore(t)
	cycleID, criterionID := seedCycleWithCriterion(t, s, ctx)

	if err := s.UpsertCriteriaReports(ctx, cycleID, 1, []CriteriaReportEntry{
		{CriterionID: criterionID, ClaimedDone: true, Evidence: "first evidence"},
	}); err != nil {
		t.Fatalf("upsert criteria: %v", err)
	}

	rows, err := s.ListCriteriaReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list criteria: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].CriterionID != criterionID || !rows[0].ClaimedDone || rows[0].Evidence != "first evidence" {
		t.Fatalf("row mismatch: %+v", rows[0])
	}

	if err := s.UpsertCriteriaReports(ctx, cycleID, 1, []CriteriaReportEntry{
		{CriterionID: criterionID, ClaimedDone: false, Evidence: "second evidence"},
	}); err != nil {
		t.Fatalf("re-upsert criteria: %v", err)
	}

	rows, err = s.ListCriteriaReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list criteria after re-upsert: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows after re-upsert = %d, want 1 (idempotent)", len(rows))
	}
	if rows[0].ClaimedDone || rows[0].Evidence != "second evidence" {
		t.Fatalf("re-upsert did not update: %+v", rows[0])
	}
}

// TestStore_UpsertVerifyReports_round_trip mirrors the criteria-side
// test and additionally pins that the verifier_kind column survives
// the round trip — the SPA renders different chips per kind so a
// silent loss here would be a regression.
func TestStore_UpsertVerifyReports_round_trip(t *testing.T) {
	s, ctx := newCycleStore(t)
	cycleID, criterionID := seedCycleWithCriterion(t, s, ctx)

	if err := s.UpsertVerifyReports(ctx, cycleID, 1, []VerifyReportEntry{
		{
			CriterionID:  criterionID,
			Verified:     true,
			VerifierKind: domain.VerifierVerifyAgent,
			Reasoning:    "tests pass",
		},
	}); err != nil {
		t.Fatalf("upsert verify: %v", err)
	}

	rows, err := s.ListVerifyReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list verify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if !rows[0].Verified || rows[0].VerifierKind != domain.VerifierVerifyAgent || rows[0].Reasoning != "tests pass" {
		t.Fatalf("row mismatch: %+v", rows[0])
	}

	if err := s.UpsertVerifyReports(ctx, cycleID, 1, []VerifyReportEntry{
		{
			CriterionID:  criterionID,
			Verified:     false,
			VerifierKind: domain.VerifierDeterministicCheck,
			Reasoning:    "still failing",
		},
	}); err != nil {
		t.Fatalf("re-upsert verify: %v", err)
	}
	rows, err = s.ListVerifyReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list verify after re-upsert: %v", err)
	}
	if len(rows) != 1 || rows[0].Verified || rows[0].VerifierKind != domain.VerifierDeterministicCheck {
		t.Fatalf("re-upsert did not update: %+v (len=%d)", rows[0], len(rows))
	}
}

// TestStore_UpsertCriteriaReports_per_attempt_rows pins the
// (cycle_id, attempt_seq, criterion_id) uniqueness contract: two
// distinct attempts produce two rows for the same (cycle, criterion),
// so retry history is preserved instead of being clobbered. This is
// the durable counterpart of the worker's previouslyPassed lock.
func TestStore_UpsertReports_per_attempt_rows(t *testing.T) {
	s, ctx := newCycleStore(t)
	cycleID, criterionID := seedCycleWithCriterion(t, s, ctx)

	for _, seq := range []int64{1, 2, 3} {
		if err := s.UpsertCriteriaReports(ctx, cycleID, seq, []CriteriaReportEntry{
			{CriterionID: criterionID, ClaimedDone: seq == 3, Evidence: "attempt"},
		}); err != nil {
			t.Fatalf("upsert criteria seq %d: %v", seq, err)
		}
		if err := s.UpsertVerifyReports(ctx, cycleID, seq, []VerifyReportEntry{
			{CriterionID: criterionID, Verified: seq == 3, VerifierKind: domain.VerifierVerifyAgent},
		}); err != nil {
			t.Fatalf("upsert verify seq %d: %v", seq, err)
		}
	}
	criteria, err := s.ListCriteriaReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list criteria: %v", err)
	}
	if len(criteria) != 3 {
		t.Fatalf("criteria rows = %d, want 3 (one per attempt)", len(criteria))
	}
	for i, row := range criteria {
		if row.AttemptSeq != int64(i+1) {
			t.Fatalf("criteria row %d attempt_seq = %d, want %d", i, row.AttemptSeq, i+1)
		}
	}
	verify, err := s.ListVerifyReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list verify: %v", err)
	}
	if len(verify) != 3 {
		t.Fatalf("verify rows = %d, want 3", len(verify))
	}
}

// TestStore_DeleteCycle_cascades_verdict_rows pins ON DELETE CASCADE
// on cycle_id: when a cycle row goes away, its verdict rows go with
// it. Without this, deleting a task (which cascades to its cycles)
// would leak orphan verdict rows forever.
func TestStore_DeleteCycle_cascades_verdict_rows(t *testing.T) {
	s, ctx := newCycleStore(t)
	cycleID, criterionID := seedCycleWithCriterion(t, s, ctx)

	if err := s.UpsertCriteriaReports(ctx, cycleID, 1, []CriteriaReportEntry{
		{CriterionID: criterionID, ClaimedDone: true, Evidence: "x"},
	}); err != nil {
		t.Fatalf("upsert criteria: %v", err)
	}
	if err := s.UpsertVerifyReports(ctx, cycleID, 1, []VerifyReportEntry{
		{CriterionID: criterionID, Verified: true, VerifierKind: domain.VerifierVerifyAgent},
	}); err != nil {
		t.Fatalf("upsert verify: %v", err)
	}

	if err := s.db.WithContext(ctx).Exec("DELETE FROM task_cycles WHERE id = ?", cycleID).Error; err != nil {
		t.Fatalf("delete cycle: %v", err)
	}

	criteria, err := s.ListCriteriaReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list criteria after delete: %v", err)
	}
	if len(criteria) != 0 {
		t.Fatalf("criteria rows after cascade = %d, want 0", len(criteria))
	}
	verify, err := s.ListVerifyReportsForCycle(ctx, cycleID)
	if err != nil {
		t.Fatalf("list verify after delete: %v", err)
	}
	if len(verify) != 0 {
		t.Fatalf("verify rows after cascade = %d, want 0", len(verify))
	}
}

// TestStore_GetCriteriaReport_returns_not_found pins the sentinel-error
// translation: callers compare against domain.ErrNotFound, never the
// driver-specific gorm.ErrRecordNotFound.
func TestStore_GetCriteriaReport_returns_not_found(t *testing.T) {
	s, ctx := newCycleStore(t)
	cycleID, _ := seedCycleWithCriterion(t, s, ctx)

	if _, err := s.GetCriteriaReport(ctx, cycleID, 1, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestStore_UpsertReports_rejects_bad_input pins the boundary
// validation: empty cycle id, empty criterion id, non-positive
// attempt_seq, and intra-batch duplicate criterion ids all surface
// as ErrInvalidInput rather than driver errors.
func TestStore_UpsertReports_rejects_bad_input(t *testing.T) {
	s, ctx := newCycleStore(t)
	cycleID, criterionID := seedCycleWithCriterion(t, s, ctx)

	cases := []struct {
		name      string
		cycleID   string
		attempt   int64
		criteria  []CriteriaReportEntry
		mustMatch string
	}{
		{"empty cycle id", "", 1, []CriteriaReportEntry{{CriterionID: criterionID}}, "cycle_id"},
		{"non-positive attempt", cycleID, 0, []CriteriaReportEntry{{CriterionID: criterionID}}, "attempt_seq"},
		{"empty criterion id", cycleID, 1, []CriteriaReportEntry{{CriterionID: ""}}, "criterion_id"},
		{
			"duplicate criterion in batch", cycleID, 1,
			[]CriteriaReportEntry{
				{CriterionID: criterionID},
				{CriterionID: criterionID},
			},
			"duplicate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.UpsertCriteriaReports(ctx, tc.cycleID, tc.attempt, tc.criteria)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if !strings.Contains(err.Error(), tc.mustMatch) {
				t.Fatalf("err = %q, want substring %q", err, tc.mustMatch)
			}
		})
	}
}
