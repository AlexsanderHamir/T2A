package postgres

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// migrateContractGitTree is the ADR-0037 contract-phase migration (Cycle 8).
// It drops the legacy columns added during the expand phase:
//
//   - git_repositories.project_id   — repositories are now globally unique on path
//   - tasks.worktree_id, tasks.branch_id — replaced by worktree_branch_id
//
// Also nulls tasks.project_id where it equals DefaultProjectID so the implicit
// default is no longer persisted.
//
// Idempotent: column drops and updates are guarded by existence checks so repeated
// runs on already-contracted databases are safe.
func migrateContractGitTree(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateContractGitTree")

	if err := contractNullDefaultProjectOnTasks(ctx, db); err != nil {
		return err
	}
	if err := contractDropGitRepoProjectID(ctx, db); err != nil {
		return err
	}
	return contractDropTaskLegacyGitColumns(ctx, db)
}

// contractNullDefaultProjectOnTasks sets tasks.project_id = NULL where it
// equals the built-in default project so the implicit default is not persisted.
func contractNullDefaultProjectOnTasks(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.contractNullDefaultProjectOnTasks")
	return db.WithContext(ctx).Exec(
		`UPDATE tasks SET project_id = NULL WHERE project_id = ?`,
		domain.DefaultProjectID,
	).Error
}

// contractDropGitRepoProjectID drops git_repositories.project_id and the
// legacy per-project unique index, leaving only the global unique on path.
func contractDropGitRepoProjectID(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.contractDropGitRepoProjectID")
	if !tableHasColumnPortable(db, "git_repositories", "project_id") {
		return nil
	}
	if isPostgres(db) {
		if err := db.WithContext(ctx).Exec(
			`DROP INDEX IF EXISTS idx_git_repo_project_path`,
		).Error; err != nil {
			return err
		}
		return db.WithContext(ctx).Exec(
			`ALTER TABLE git_repositories DROP COLUMN IF EXISTS project_id`,
		).Error
	}
	return dropColumnSQLite(ctx, db, "git_repositories", "project_id")
}

// contractDropTaskLegacyGitColumns drops the ADR-0037 expand-phase pair
// tasks.worktree_id + tasks.branch_id (replaced by worktree_branch_id during
// that era). ADR-0039 reintroduces tasks.worktree_id with different semantics;
// skip when tasks.branch_id is absent (fresh rev-4 or post-fixed migration).
func contractDropTaskLegacyGitColumns(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.contractDropTaskLegacyGitColumns")
	if !tableHasColumnPortable(db, "tasks", "branch_id") {
		return nil
	}
	if !tableHasColumnPortable(db, "tasks", "worktree_id") {
		return nil
	}
	if isPostgres(db) {
		if err := db.WithContext(ctx).Exec(
			`ALTER TABLE tasks DROP COLUMN IF EXISTS worktree_id`,
		).Error; err != nil {
			return err
		}
		return db.WithContext(ctx).Exec(
			`ALTER TABLE tasks DROP COLUMN IF EXISTS branch_id`,
		).Error
	}
	if err := dropColumnSQLite(ctx, db, "tasks", "worktree_id"); err != nil {
		return err
	}
	return dropColumnSQLite(ctx, db, "tasks", "branch_id")
}

//funclogmeasure:skip category=hot-path reason="Schema introspection helper; called at boot in Migrate."
func isPostgres(db *gorm.DB) bool {
	return db.Dialector != nil && db.Dialector.Name() == "postgres"
}

// dropColumnSQLite drops a column via ALTER TABLE DROP COLUMN, supported
// since SQLite 3.35.0 (2021-03-12). All Go SQLite drivers used by this
// project bundle a version >= 3.35.
func dropColumnSQLite(ctx context.Context, db *gorm.DB, table, column string) error {
	slog.Debug("trace", "operation", "postgres.dropColumnSQLite", "table", table, "column", column)
	return db.WithContext(ctx).Exec("ALTER TABLE " + table + " DROP COLUMN " + column).Error
}
