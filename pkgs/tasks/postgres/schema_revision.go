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

// SchemaRevision is bumped in the same PR as any change to domain models or
// idempotent post-AutoMigrate steps in Migrate.
const SchemaRevision = 1

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

// Remediation returns operator guidance when drift fails readiness.
//
//funclogmeasure:skip category=hot-path reason="Pure constant accessor; drift is traced at taskapi startup and GET /health/ready."
func (r SchemaDriftReport) Remediation() string {
	return SchemaDriftRemediation
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
