package store

import (
	"context"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// hasRunningTaskOnGitTarget reports whether any task with status running
// references the target id as a worktree or descendant of a repository.
//
//funclogmeasure:skip category=hot-path reason="DB read helper; operation trace is emitted by the calling delete chokepoint."
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
     OR worktree_id IN (
          SELECT id FROM git_worktrees WHERE repository_id = ?
        )
  )`, domain.StatusRunning, targetID, targetID).Scan(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

//funclogmeasure:skip category=hot-path reason="DB read helper; operation trace is emitted by the calling delete chokepoint."
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

//funclogmeasure:skip category=hot-path reason="DB read helper; operation trace is emitted by ReconcileGitRepository chokepoint."
func hasAnyTaskOnWorktree(ctx context.Context, db *gorm.DB, worktreeID string) (bool, error) {
	if worktreeID == "" {
		return false, nil
	}
	var n int64
	err := db.WithContext(ctx).Raw(`
SELECT COUNT(*) FROM tasks WHERE worktree_id = ?`, worktreeID).Scan(&n).Error
	return n > 0, err
}
