package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/reports"
)

// CriteriaReportEntry is the public re-export of the per-criterion
// criteria-report payload. The alias keeps the worker call-site
// (UpsertCriteriaReports) shielded from the internal reports package.
type CriteriaReportEntry = reports.CriteriaEntry

// VerifyReportEntry is the verify-report counterpart of
// CriteriaReportEntry.
type VerifyReportEntry = reports.VerifyEntry

// UpsertCriteriaReports persists one batch of per-criterion
// criteria-report rows for (cycleID, attemptSeq). See
// reports.UpsertCriteriaReports for the idempotency contract.
func (s *Store) UpsertCriteriaReports(ctx context.Context, cycleID string, attemptSeq int64, entries []CriteriaReportEntry) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpsertCriteriaReports")
	return reports.UpsertCriteriaReports(ctx, s.db, cycleID, attemptSeq, entries)
}

// UpsertVerifyReports is the verify-report counterpart of
// UpsertCriteriaReports.
func (s *Store) UpsertVerifyReports(ctx context.Context, cycleID string, attemptSeq int64, entries []VerifyReportEntry) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpsertVerifyReports")
	return reports.UpsertVerifyReports(ctx, s.db, cycleID, attemptSeq, entries)
}

// ListCriteriaReportsForCycle returns every persisted
// criteria-report row for cycleID ordered by (attempt_seq ASC,
// criterion_id ASC). Pre-PR2 cycles return an empty slice.
func (s *Store) ListCriteriaReportsForCycle(ctx context.Context, cycleID string) ([]domain.TaskCycleCriteriaReport, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListCriteriaReportsForCycle")
	return reports.ListCriteriaReportsForCycle(ctx, s.db, cycleID)
}

// ListVerifyReportsForCycle is the verify counterpart of
// ListCriteriaReportsForCycle.
func (s *Store) ListVerifyReportsForCycle(ctx context.Context, cycleID string) ([]domain.TaskCycleVerifyReport, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListVerifyReportsForCycle")
	return reports.ListVerifyReportsForCycle(ctx, s.db, cycleID)
}

// GetCriteriaReport returns the criteria-report row for
// (cycleID, attemptSeq, criterionID); ErrNotFound when missing.
// Exposed primarily for store tests; the handler reads via the bulk
// list endpoints.
func (s *Store) GetCriteriaReport(ctx context.Context, cycleID string, attemptSeq int64, criterionID string) (*domain.TaskCycleCriteriaReport, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetCriteriaReport")
	return reports.GetCriteriaReport(ctx, s.db, cycleID, attemptSeq, criterionID)
}
