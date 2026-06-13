package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// migrateRemoveTaskType drops the legacy tasks.task_type column and its check
// constraint. Idempotent — safe on fresh installs and upgraded databases.
func migrateRemoveTaskType(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateRemoveTaskType")
	if db.Dialector == nil {
		return nil
	}
	switch db.Dialector.Name() {
	case "postgres":
		if err := db.WithContext(ctx).Exec(`ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_task_type`).Error; err != nil {
			return fmt.Errorf("drop tasks task_type constraint: %w", err)
		}
		if err := db.WithContext(ctx).Exec(`ALTER TABLE tasks DROP COLUMN IF EXISTS task_type`).Error; err != nil {
			return fmt.Errorf("drop tasks.task_type: %w", err)
		}
	case "sqlite":
		ok, err := tableHasColumn(ctx, db, "tasks", "task_type")
		if err != nil {
			return fmt.Errorf("probe tasks.task_type: %w", err)
		}
		if ok {
			if err := db.WithContext(ctx).Exec(`ALTER TABLE tasks DROP COLUMN task_type`).Error; err != nil {
				return fmt.Errorf("drop tasks.task_type: %w", err)
			}
		}
	}
	return nil
}
