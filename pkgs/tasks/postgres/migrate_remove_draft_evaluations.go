package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// migrateRemoveDraftEvaluations drops the legacy task_draft_evaluations table.
// Idempotent — safe on fresh installs (no table) and upgraded databases.
func migrateRemoveDraftEvaluations(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateRemoveDraftEvaluations")
	if err := db.WithContext(ctx).Exec(`DROP TABLE IF EXISTS task_draft_evaluations`).Error; err != nil {
		return fmt.Errorf("drop task_draft_evaluations: %w", err)
	}
	return nil
}
