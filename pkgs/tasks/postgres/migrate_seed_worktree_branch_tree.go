package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// migrateSeedWorktreeBranchTree is the ADR-0037 expand-phase backfill. It is
// additive and idempotent: it never drops columns or rows, so it is safe on
// fresh and upgraded databases and across repeated boots. The contract-phase
// removals (drop git_repositories.project_id, drop tasks.worktree_id/branch_id,
// null default-project ownership, enforce global path uniqueness) are applied
// separately in a later release.
//
// Steps:
//  1. Set projects.repository_id from each project's legacy per-project
//     git_repositories row (skips projects already linked or with no repo).
//  2. Seed worktree_branches from existing task (worktree_id, branch_id) pairs.
//  3. Backfill tasks.worktree_branch_id from those pairs.
func migrateSeedWorktreeBranchTree(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateSeedWorktreeBranchTree")
	if err := backfillProjectRepository(ctx, db); err != nil {
		return err
	}
	return backfillTaskWorktreeBranch(ctx, db)
}

// backfillProjectRepository links each project to one of its legacy
// per-project repositories. After the C8 contract migration the
// project_id column on git_repositories is dropped, so this function
// becomes a no-op on post-C8 databases.
func backfillProjectRepository(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillProjectRepository")
	if !tableHasColumnPortable(db, "git_repositories", "project_id") {
		return nil
	}
	var projects []model.Project
	if err := db.WithContext(ctx).
		Where("repository_id IS NULL").
		Find(&projects).Error; err != nil {
		return err
	}
	for i := range projects {
		var repoID string
		err := db.WithContext(ctx).Raw(
			`SELECT id FROM git_repositories WHERE project_id = ? ORDER BY created_at ASC LIMIT 1`,
			projects[i].ID,
		).Scan(&repoID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || repoID == "" {
			continue
		}
		if err != nil {
			return err
		}
		if err := db.WithContext(ctx).Model(&model.Project{}).
			Where("id = ?", projects[i].ID).
			Update("repository_id", repoID).Error; err != nil {
			return err
		}
	}
	return nil
}

// worktreeBranchPair is a distinct (worktree, branch) binding observed on tasks.
type worktreeBranchPair struct {
	WorktreeID string `gorm:"column:worktree_id"`
	BranchID   string `gorm:"column:branch_id"`
}

// backfillTaskWorktreeBranch seeds worktree_branches from legacy task bindings
// and points tasks.worktree_branch_id at the resulting association rows. Pairs
// whose worktree or branch no longer exist are skipped to respect the
// association's foreign keys.
//
// After the C8 contract migration the legacy columns are dropped, so this
// function becomes a no-op on fresh databases.
func backfillTaskWorktreeBranch(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillTaskWorktreeBranch")
	if !tableHasColumnPortable(db, "tasks", "worktree_id") {
		return nil
	}
	var pairs []worktreeBranchPair
	if err := db.WithContext(ctx).Raw(
		`SELECT worktree_id, branch_id FROM tasks WHERE worktree_id IS NOT NULL AND branch_id IS NOT NULL GROUP BY worktree_id, branch_id`,
	).Scan(&pairs).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, p := range pairs {
		ok, err := worktreeBranchEndpointsExist(ctx, db, p)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		wbID, err := ensureWorktreeBranch(ctx, db, p, now)
		if err != nil {
			return err
		}
		if err := db.WithContext(ctx).Exec(
			`UPDATE tasks SET worktree_branch_id = ? WHERE worktree_id = ? AND branch_id = ? AND worktree_branch_id IS NULL`,
			wbID, p.WorktreeID, p.BranchID,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func worktreeBranchEndpointsExist(ctx context.Context, db *gorm.DB, p worktreeBranchPair) (bool, error) {
	slog.Debug("trace", "operation", "postgres.worktreeBranchEndpointsExist")
	var wt int64
	if err := db.WithContext(ctx).Model(&model.GitWorktree{}).
		Where("id = ?", p.WorktreeID).Count(&wt).Error; err != nil {
		return false, err
	}
	var br int64
	if err := db.WithContext(ctx).Model(&model.GitBranch{}).
		Where("id = ?", p.BranchID).Count(&br).Error; err != nil {
		return false, err
	}
	return wt > 0 && br > 0, nil
}

// ensureWorktreeBranch upserts the association for a pair and returns its id.
func ensureWorktreeBranch(ctx context.Context, db *gorm.DB, p worktreeBranchPair, now time.Time) (string, error) {
	slog.Debug("trace", "operation", "postgres.ensureWorktreeBranch")
	row := model.WorktreeBranch{
		ID:         uuid.NewString(),
		WorktreeID: p.WorktreeID,
		BranchID:   p.BranchID,
		CreatedAt:  now,
	}
	if err := db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error; err != nil {
		return "", err
	}
	var existing model.WorktreeBranch
	if err := db.WithContext(ctx).
		Where("worktree_id = ? AND branch_id = ?", p.WorktreeID, p.BranchID).
		First(&existing).Error; err != nil {
		return "", err
	}
	return existing.ID, nil
}
