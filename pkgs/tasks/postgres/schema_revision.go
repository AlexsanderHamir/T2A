package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SchemaRevision is bumped in the same PR as any change to store persistence
// models, domain types, or idempotent post-AutoMigrate steps in Migrate.
//
// Rev 2 (ADR-0037 expand phase): adds worktree_branches, git_worktrees
// .active_branch_id, projects.repository_id, tasks.worktree_branch_id and their
// idempotent backfill.
//
// Rev 3 (ADR-0037 contract phase / Cycle 8): drops git_repositories.project_id,
// tasks.worktree_id, tasks.branch_id; nulls default-project ownership on tasks.
//
// Rev 4 (ADR-0039 phase 6 cutover): AutoMigrate targets store/model types;
// domain structs are persistence-agnostic (no GORM tags).
const SchemaRevision = 4

const schemaMetaRowID = 1

// SchemaDriftRemediation tells operators how to apply pending schema changes.
const SchemaDriftRemediation = "run dbcheck -migrate or scripts/migrate.*"

// SchemaDriftStatus classifies code vs database schema revision alignment.
type SchemaDriftStatus string

const (
	SchemaDriftOK        SchemaDriftStatus = "ok"
	SchemaDriftPending   SchemaDriftStatus = "pending"
	SchemaDriftDowngrade SchemaDriftStatus = "downgrade"
)

// SchemaMeta records the schema revision last applied by postgres.Migrate.
type SchemaMeta struct {
	ID        int       `gorm:"primaryKey"`
	Revision  int       `gorm:"not null"`
	AppliedAt time.Time `gorm:"not null"`
}

// SchemaDriftReport is the result of comparing code SchemaRevision to schema_meta.
type SchemaDriftReport struct {
	Status       SchemaDriftStatus
	CodeRevision int
	DBRevision   int
}

// OperatorMessage is plain-language guidance for stderr and fatal startup errors.
//
//funclogmeasure:skip category=hot-path reason="Pure string formatter; drift is traced at taskapi startup."
func (r SchemaDriftReport) OperatorMessage() string {
	switch r.Status {
	case SchemaDriftPending:
		if r.DBRevision == 0 {
			return "Database schema has not been migrated yet. Apply schema migrate before starting taskapi."
		}
		return "Database schema is out of date for this build. Apply schema migrate before starting taskapi."
	case SchemaDriftDowngrade:
		return "This taskapi build is older than the database schema. Deploy a matching release before starting taskapi."
	default:
		return ""
	}
}

// Remediation returns operator guidance when drift fails readiness.
//
//funclogmeasure:skip category=hot-path reason="Pure constant accessor; drift is traced at taskapi startup and GET /health/ready."
func (r SchemaDriftReport) Remediation() string {
	switch r.Status {
	case SchemaDriftDowngrade:
		return "deploy a taskapi build that matches the database schema"
	default:
		return SchemaDriftRemediation
	}
}

// RemediationCLI returns a one-line shell command for stderr when migrate applies.
//
//funclogmeasure:skip category=hot-path reason="Pure string formatter; drift is traced at taskapi startup."
func (r SchemaDriftReport) RemediationCLI() string {
	switch r.Status {
	case SchemaDriftDowngrade:
		return "Deploy a taskapi build that matches the database schema."
	default:
		return "Run: .\\scripts\\migrate.ps1   or   go run ./cmd/dbcheck -migrate"
	}
}

// FailsReadiness reports whether GET /health/ready should return 503 for schema.
//
//funclogmeasure:skip category=hot-path reason="Pure predicate; readiness handler traces the HTTP boundary."
func (r SchemaDriftReport) FailsReadiness() bool {
	return r.Status == SchemaDriftPending || r.Status == SchemaDriftDowngrade
}

// DefaultDevStartupGrace is added to DefaultMigrateTimeout for dev script port waits
// when migrate runs before taskapi (scripts/dev.* --migrate sugar).
const DefaultDevStartupGrace = 30 * time.Second

// DefaultDevReadinessTimeout returns how long dev scripts wait for taskapi to listen.
//
//funclogmeasure:skip category=hot-path reason="Pure duration helper for devconfig CLI; no I/O boundary."
func DefaultDevReadinessTimeout() time.Duration {
	return DefaultMigrateTimeout + DefaultDevStartupGrace
}

// CheckSchemaDrift compares SchemaRevision to the revision stored in schema_meta.
func CheckSchemaDrift(ctx context.Context, db *gorm.DB) (SchemaDriftReport, error) {
	slog.Debug("trace", "operation", "postgres.CheckSchemaDrift")
	report := SchemaDriftReport{
		Status:       SchemaDriftPending,
		CodeRevision: SchemaRevision,
		DBRevision:   0,
	}
	if err := db.WithContext(ctx).AutoMigrate(&SchemaMeta{}); err != nil {
		return report, fmt.Errorf("automigrate schema_meta: %w", err)
	}
	var meta SchemaMeta
	err := db.WithContext(ctx).First(&meta, schemaMetaRowID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return report, nil
	}
	if err != nil {
		return report, fmt.Errorf("read schema_meta: %w", err)
	}
	report.DBRevision = meta.Revision
	switch {
	case meta.Revision < SchemaRevision:
		report.Status = SchemaDriftPending
	case meta.Revision > SchemaRevision:
		report.Status = SchemaDriftDowngrade
	default:
		report.Status = SchemaDriftOK
	}
	return report, nil
}

// RecordSchemaRevision upserts schema_meta after a successful Migrate.
func RecordSchemaRevision(ctx context.Context, db *gorm.DB, at time.Time) error {
	slog.Debug("trace", "operation", "postgres.RecordSchemaRevision", "revision", SchemaRevision)
	if err := db.WithContext(ctx).AutoMigrate(&SchemaMeta{}); err != nil {
		return fmt.Errorf("automigrate schema_meta: %w", err)
	}
	row := SchemaMeta{
		ID:        schemaMetaRowID,
		Revision:  SchemaRevision,
		AppliedAt: at.UTC(),
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"revision", "applied_at"}),
	}).Create(&row).Error; err != nil {
		return fmt.Errorf("upsert schema_meta: %w", err)
	}
	return nil
}
