package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// migrateDropRepoRootColumn removes the deprecated app_settings.repo_root column
// after migrateRepoRootToGitRepository has backfilled git_repositories.
func migrateDropRepoRootColumn(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateDropRepoRootColumn")
	if db.Dialector == nil {
		return nil
	}
	switch db.Dialector.Name() {
	case "postgres":
		if err := db.WithContext(ctx).Exec(`ALTER TABLE app_settings DROP COLUMN IF EXISTS repo_root`).Error; err != nil {
			return fmt.Errorf("drop app_settings.repo_root: %w", err)
		}
	case "sqlite":
		ok, err := tableHasColumn(ctx, db, "app_settings", "repo_root")
		if err != nil {
			return fmt.Errorf("probe app_settings.repo_root: %w", err)
		}
		if ok {
			if err := db.WithContext(ctx).Exec(`ALTER TABLE app_settings DROP COLUMN repo_root`).Error; err != nil {
				return fmt.Errorf("drop app_settings.repo_root: %w", err)
			}
		}
	}
	return nil
}
