package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// migrateExpandFixedWorktreeBranch runs before AutoMigrate on databases upgraded
// from schema rev 3. GORM would otherwise ADD branch_id NOT NULL in one step while
// existing git_worktrees rows are still null (PostgreSQL SQLSTATE 23502).
func migrateExpandFixedWorktreeBranch(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateExpandFixedWorktreeBranch")
	if !db.Migrator().HasTable("git_worktrees") {
		return nil
	}
	if !tableHasColumnPortable(db, "git_worktrees", "branch_id") {
		if err := ensureNullableTextColumn(ctx, db, "git_worktrees", "branch_id"); err != nil {
			return fmt.Errorf("add git_worktrees.branch_id: %w", err)
		}
	}
	return backfillFixedWorktreeBranchData(ctx, db)
}

// migrateFixedWorktreeBranch is the ADR-0039 migration (schema rev 4).
// Collapses worktree_branches into git_worktrees.branch_id and tasks.worktree_id.
func migrateFixedWorktreeBranch(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateFixedWorktreeBranch")

	if err := backfillFixedWorktreeBranchData(ctx, db); err != nil {
		return err
	}
	if err := dropLegacyWorktreeBranchArtifacts(ctx, db); err != nil {
		return err
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func backfillFixedWorktreeBranchData(ctx context.Context, db *gorm.DB) error {
	if tableHasColumnPortable(db, "worktree_branches", "id") {
		if err := backfillWorktreeBranchID(ctx, db); err != nil {
			return err
		}
		if err := backfillTaskWorktreeID(ctx, db); err != nil {
			return err
		}
	}
	if err := backfillWorktreeBranchIDFromActive(ctx, db); err != nil {
		return err
	}
	return backfillMainWorktreeDefaultBranch(ctx, db)
}

func ensureNullableTextColumn(ctx context.Context, db *gorm.DB, table, column string) error {
	slog.Debug("trace", "operation", "postgres.ensureNullableTextColumn", "table", table, "column", column)
	if tableHasColumnPortable(db, table, column) {
		return nil
	}
	if isPostgres(db) {
		return db.WithContext(ctx).Exec(fmt.Sprintf(
			`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s text`, table, column,
		)).Error
	}
	return db.WithContext(ctx).Exec(fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN %s text`, table, column,
	)).Error
}

func backfillWorktreeBranchID(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillWorktreeBranchID")
	if !tableHasColumnPortable(db, "git_worktrees", "branch_id") {
		return nil
	}
	if isPostgres(db) {
		return db.WithContext(ctx).Exec(`
UPDATE git_worktrees AS wt
   SET branch_id = sub.branch_id
  FROM (
    SELECT DISTINCT ON (worktree_id) worktree_id, branch_id
      FROM worktree_branches
     ORDER BY worktree_id, created_at ASC
  ) AS sub
 WHERE wt.id = sub.worktree_id
   AND (wt.branch_id IS NULL OR wt.branch_id = '')`).Error
	}
	return db.WithContext(ctx).Exec(`
UPDATE git_worktrees
   SET branch_id = (
     SELECT wb.branch_id FROM worktree_branches wb
      WHERE wb.worktree_id = git_worktrees.id
      ORDER BY wb.created_at ASC LIMIT 1
   )
 WHERE (branch_id IS NULL OR branch_id = '')
   AND EXISTS (SELECT 1 FROM worktree_branches wb WHERE wb.worktree_id = git_worktrees.id)`).Error
}

func backfillTaskWorktreeID(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillTaskWorktreeID")
	if !tableHasColumnPortable(db, "tasks", "worktree_id") {
		return nil
	}
	if !tableHasColumnPortable(db, "tasks", "worktree_branch_id") {
		return nil
	}
	if isPostgres(db) {
		return db.WithContext(ctx).Exec(`
UPDATE tasks AS t
   SET worktree_id = wb.worktree_id
  FROM worktree_branches AS wb
 WHERE t.worktree_branch_id = wb.id
   AND (t.worktree_id IS NULL OR t.worktree_id = '')`).Error
	}
	return db.WithContext(ctx).Exec(`
UPDATE tasks
   SET worktree_id = (
     SELECT wb.worktree_id FROM worktree_branches wb
      WHERE wb.id = tasks.worktree_branch_id
   )
 WHERE worktree_branch_id IS NOT NULL
   AND (worktree_id IS NULL OR worktree_id = '')`).Error
}

func backfillWorktreeBranchIDFromActive(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillWorktreeBranchIDFromActive")
	if !tableHasColumnPortable(db, "git_worktrees", "branch_id") {
		return nil
	}
	if !tableHasColumnPortable(db, "git_worktrees", "active_branch_id") {
		return nil
	}
	return db.WithContext(ctx).Exec(`
UPDATE git_worktrees
   SET branch_id = active_branch_id
 WHERE (branch_id IS NULL OR branch_id = '')
   AND active_branch_id IS NOT NULL
   AND active_branch_id <> ''`).Error
}

func backfillMainWorktreeDefaultBranch(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillMainWorktreeDefaultBranch")
	if !tableHasColumnPortable(db, "git_worktrees", "branch_id") {
		return nil
	}
	if isPostgres(db) {
		return db.WithContext(ctx).Exec(`
UPDATE git_worktrees AS wt
   SET branch_id = b.id
  FROM git_repositories AS r
  JOIN git_branches AS b ON b.repository_id = r.id AND b.name = r.default_branch
 WHERE wt.repository_id = r.id
   AND wt.is_main = true
   AND (wt.branch_id IS NULL OR wt.branch_id = '')`).Error
	}
	return db.WithContext(ctx).Exec(`
UPDATE git_worktrees
   SET branch_id = (
     SELECT b.id FROM git_branches b
      JOIN git_repositories r ON r.id = b.repository_id
      WHERE b.repository_id = git_worktrees.repository_id
        AND b.name = r.default_branch
      LIMIT 1
   )
 WHERE is_main = 1
   AND (branch_id IS NULL OR branch_id = '')`).Error
}

func dropLegacyWorktreeBranchArtifacts(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.dropLegacyWorktreeBranchArtifacts")

	if tableHasColumnPortable(db, "tasks", "worktree_branch_id") {
		if isPostgres(db) {
			if err := db.WithContext(ctx).Exec(`ALTER TABLE tasks DROP COLUMN IF EXISTS worktree_branch_id`).Error; err != nil {
				return err
			}
		} else if err := dropColumnSQLite(ctx, db, "tasks", "worktree_branch_id"); err != nil {
			return err
		}
	}

	if tableHasColumnPortable(db, "git_worktrees", "active_branch_id") {
		if isPostgres(db) {
			if err := db.WithContext(ctx).Exec(`ALTER TABLE git_worktrees DROP COLUMN IF EXISTS active_branch_id`).Error; err != nil {
				return err
			}
		} else if err := dropColumnSQLite(ctx, db, "git_worktrees", "active_branch_id"); err != nil {
			return err
		}
	}

	if db.Migrator().HasTable("worktree_branches") {
		if err := db.WithContext(ctx).Migrator().DropTable("worktree_branches"); err != nil {
			return err
		}
	}
	return nil
}
