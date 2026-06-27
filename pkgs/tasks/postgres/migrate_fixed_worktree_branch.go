package postgres

import (
	"context"
	"log/slog"

	"gorm.io/gorm"
)

// migrateFixedWorktreeBranch is the ADR-0039 migration (schema rev 4).
// Collapses worktree_branches into git_worktrees.branch_id and tasks.worktree_id.
func migrateFixedWorktreeBranch(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateFixedWorktreeBranch")

	if tableHasColumnPortable(db, "worktree_branches", "id") {
		if err := backfillWorktreeBranchID(ctx, db); err != nil {
			return err
		}
		if err := backfillTaskWorktreeID(ctx, db); err != nil {
			return err
		}
	}

	if err := dropLegacyWorktreeBranchArtifacts(ctx, db); err != nil {
		return err
	}
	return nil
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
