// Package reports persists per-criterion verdict evidence (the
// criteria-report.json + verify-report.json side-channel files) into
// task_cycle_criteria_reports / task_cycle_verify_reports for durable
// query / SPA render / support inspection.
//
// Wire shape stays the file format: the worker parses the JSON the
// agent CLI wrote, then bulk-upserts one row per criterion per
// (cycle, attempt). Idempotency is keyed by (cycle_id, attempt_seq,
// criterion_id) so a parse-then-store retry after a transient DB
// error never duplicates rows or shifts a verdict.
package reports

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const logCmd = "taskapi"

// CriteriaEntry is the per-criterion payload for one execute attempt's
// criteria-report.json row, in the shape the worker hands to
// UpsertCriteriaReports. CriterionID is the stable checklist item id
// the agent referenced in its report.
type CriteriaEntry struct {
	CriterionID string
	ClaimedDone bool
	Evidence    string
}

// VerifyEntry is the per-criterion payload for one verify attempt's
// verify-report.json row. VerifierKind records how the verdict was
// reached (deterministic_check / verify_agent / agent_self), matching
// the verifier_kind values used on task_checklist_completions so the
// SPA can render the same chip in both surfaces.
type VerifyEntry struct {
	CriterionID  string
	Verified     bool
	VerifierKind domain.VerifierKind
	Reasoning    string
}

// UpsertCriteriaReports inserts or updates rows for every entry in one
// (cycleID, attemptSeq) batch. Idempotent against the
// (cycle_id, attempt_seq, criterion_id) unique index, so a worker
// retry after a transient store error simply rewrites the same row.
// Empty entries returns nil without a query.
//
// Bulk over per-row INSERT because every criterion in a cycle's
// criteria-report.json is written together; one round-trip is much
// cheaper than N at scale (a few hundred criteria per cycle is well
// inside SQLite's variable-bind cap, and Postgres handles thousands
// without ceremony). When N grows past that we can chunk inside this
// function without changing the contract.
func UpsertCriteriaReports(ctx context.Context, db *gorm.DB, cycleID string, attemptSeq int64, entries []CriteriaEntry) error {
	defer kernel.DeferLatency(kernel.OpUpsertCriteriaReports)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.reports.UpsertCriteriaReports",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "entry_count", len(entries))
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if attemptSeq <= 0 {
		return fmt.Errorf("%w: attempt_seq must be positive", domain.ErrInvalidInput)
	}
	if len(entries) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]domain.TaskCycleCriteriaReport, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		id := strings.TrimSpace(e.CriterionID)
		if id == "" {
			return fmt.Errorf("%w: criterion_id", domain.ErrInvalidInput)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: duplicate criterion_id %s", domain.ErrInvalidInput, id)
		}
		seen[id] = struct{}{}
		rows = append(rows, domain.TaskCycleCriteriaReport{
			ID:          uuid.NewString(),
			CycleID:     cycleID,
			AttemptSeq:  attemptSeq,
			CriterionID: id,
			ClaimedDone: e.ClaimedDone,
			Evidence:    e.Evidence,
			WrittenAt:   now,
		})
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cycle_id"}, {Name: "attempt_seq"}, {Name: "criterion_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"claimed_done", "evidence", "written_at",
		}),
	}).Omit("Cycle", "Criterion").Create(&rows).Error
	if err != nil {
		return fmt.Errorf("upsert criteria reports: %w", err)
	}
	return nil
}

// UpsertVerifyReports is the verify-report counterpart of
// UpsertCriteriaReports. Same idempotency contract, same bulk strategy,
// same key.
func UpsertVerifyReports(ctx context.Context, db *gorm.DB, cycleID string, attemptSeq int64, entries []VerifyEntry) error {
	defer kernel.DeferLatency(kernel.OpUpsertVerifyReports)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.reports.UpsertVerifyReports",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "entry_count", len(entries))
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if attemptSeq <= 0 {
		return fmt.Errorf("%w: attempt_seq must be positive", domain.ErrInvalidInput)
	}
	if len(entries) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]domain.TaskCycleVerifyReport, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		id := strings.TrimSpace(e.CriterionID)
		if id == "" {
			return fmt.Errorf("%w: criterion_id", domain.ErrInvalidInput)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: duplicate criterion_id %s", domain.ErrInvalidInput, id)
		}
		seen[id] = struct{}{}
		rows = append(rows, domain.TaskCycleVerifyReport{
			ID:           uuid.NewString(),
			CycleID:      cycleID,
			AttemptSeq:   attemptSeq,
			CriterionID:  id,
			Verified:     e.Verified,
			VerifierKind: e.VerifierKind,
			Reasoning:    e.Reasoning,
			WrittenAt:    now,
		})
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cycle_id"}, {Name: "attempt_seq"}, {Name: "criterion_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"verified", "verifier_kind", "reasoning", "written_at",
		}),
	}).Omit("Cycle", "Criterion").Create(&rows).Error
	if err != nil {
		return fmt.Errorf("upsert verify reports: %w", err)
	}
	return nil
}

// ListCriteriaReportsForCycle returns every persisted criteria-report
// row for cycleID ordered by (attempt_seq ASC, criterion_id ASC) so
// the SPA can render the per-attempt timeline directly. Pre-PR2
// cycles return an empty slice (no rows mirrored); the handler is
// responsible for treating missing rows as "feature not yet
// available" rather than 404.
func ListCriteriaReportsForCycle(ctx context.Context, db *gorm.DB, cycleID string) ([]domain.TaskCycleCriteriaReport, error) {
	defer kernel.DeferLatency(kernel.OpListCriteriaReportsForCycle)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.reports.ListCriteriaReportsForCycle",
		"cycle_id", cycleID)
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskCycleCriteriaReport
	if err := db.WithContext(ctx).
		Where("cycle_id = ?", cycleID).
		Order("attempt_seq ASC, criterion_id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list criteria reports: %w", err)
	}
	return rows, nil
}

// ListVerifyReportsForCycle is the verify counterpart of
// ListCriteriaReportsForCycle.
func ListVerifyReportsForCycle(ctx context.Context, db *gorm.DB, cycleID string) ([]domain.TaskCycleVerifyReport, error) {
	defer kernel.DeferLatency(kernel.OpListVerifyReportsForCycle)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.reports.ListVerifyReportsForCycle",
		"cycle_id", cycleID)
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskCycleVerifyReport
	if err := db.WithContext(ctx).
		Where("cycle_id = ?", cycleID).
		Order("attempt_seq ASC, criterion_id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list verify reports: %w", err)
	}
	return rows, nil
}

// GetCriteriaReport returns one row by (cycleID, attemptSeq,
// criterionID); ErrNotFound when missing. Used by tests and a future
// per-criterion drill-down endpoint.
func GetCriteriaReport(ctx context.Context, db *gorm.DB, cycleID string, attemptSeq int64, criterionID string) (*domain.TaskCycleCriteriaReport, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.reports.GetCriteriaReport",
		"cycle_id", cycleID, "attempt_seq", attemptSeq, "criterion_id", criterionID)
	cycleID = strings.TrimSpace(cycleID)
	criterionID = strings.TrimSpace(criterionID)
	if cycleID == "" || criterionID == "" || attemptSeq <= 0 {
		return nil, fmt.Errorf("%w: report key", domain.ErrInvalidInput)
	}
	var row domain.TaskCycleCriteriaReport
	err := db.WithContext(ctx).
		Where("cycle_id = ? AND attempt_seq = ? AND criterion_id = ?", cycleID, attemptSeq, criterionID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get criteria report: %w", err)
	}
	return &row, nil
}
