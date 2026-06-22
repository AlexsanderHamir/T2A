package store

import (
	"context"
	"errors"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// hasRunningTaskOnGitTarget reports whether any task with status running
// references the target id as a worktree, branch, or descendant of a repository.
func hasRunningTaskOnGitTarget(ctx context.Context, db *gorm.DB, targetID string) (bool, error) {
	if targetID == "" {
		return false, nil
	}
	var n int64
	err := db.WithContext(ctx).Raw(`
SELECT COUNT(*) FROM tasks
WHERE status = ?
  AND (
        worktree_id = ?
     OR branch_id = ?
     OR worktree_id IN (SELECT id FROM git_worktrees WHERE repository_id = ?)
     OR branch_id IN (SELECT id FROM git_branches WHERE repository_id = ?)
  )`, domain.StatusRunning, targetID, targetID, targetID, targetID).Scan(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func guardNoRunningTask(ctx context.Context, db *gorm.DB, targetID string) error {
	ok, err := hasRunningTaskOnGitTarget(ctx, db, targetID)
	if err != nil {
		return err
	}
	if ok {
		return domain.NewGitErr(domain.GitCodeHasRunningTask, "a task is running on this git target")
	}
	return nil
}

func hasAnyTaskOnWorktree(ctx context.Context, db *gorm.DB, worktreeID string) (bool, error) {
	if worktreeID == "" {
		return false, nil
	}
	var n int64
	err := db.WithContext(ctx).Model(&domain.Task{}).
		Where("worktree_id = ?", worktreeID).
		Count(&n).Error
	return n > 0, err
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return errors.Is(err, gorm.ErrDuplicatedKey) ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique constraint failed")
}
