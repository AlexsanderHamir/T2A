package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateGitWorktreeInput adds a worktree on disk and persists the row.
type CreateGitWorktreeInput struct {
	Path         string
	Name         string
	Branch       string
	CreateBranch bool
	StartPoint   string
	ForceRemove  bool
}

// ListGitWorktrees returns worktrees for a repository.
func (s *Store) ListGitWorktrees(ctx context.Context, projectID, repoID string) ([]domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListGitWorktrees")
	if _, err := s.GetGitRepository(ctx, projectID, repoID); err != nil {
		return nil, err
	}
	var rows []domain.GitWorktree
	err := s.db.WithContext(ctx).
		Where("repository_id = ?", repoID).
		Order("is_main DESC, created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list git worktrees: %w", err)
	}
	return rows, nil
}

// GetGitWorktree returns one worktree by ID. The projectID parameter is
// accepted for API compatibility but ignored — repositories are global.
func (s *Store) GetGitWorktree(ctx context.Context, projectID, worktreeID string) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetGitWorktree")
	var row domain.GitWorktree
	err := s.db.WithContext(ctx).
		Where("id = ?", worktreeID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitWorktree{}, domain.NewGitErr(domain.GitCodeWorktreeNotFound, "worktree not found")
		}
		return domain.GitWorktree{}, fmt.Errorf("get git worktree: %w", err)
	}
	return row, nil
}

// CreateGitWorktree adds a linked worktree via git and inserts a row.
func (s *Store) CreateGitWorktree(ctx context.Context, projectID, repoID string, input CreateGitWorktreeInput, gitSvc gitwork.Service) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.CreateGitWorktree")
	repo, err := s.GetGitRepository(ctx, projectID, repoID)
	if err != nil {
		return domain.GitWorktree{}, err
	}
	return s.createGitWorktreeOnRepo(ctx, repo, input, gitSvc)
}

// DeleteGitWorktree removes a worktree from disk and the database.
func (s *Store) DeleteGitWorktree(ctx context.Context, projectID, worktreeID string, force bool, gitSvc gitwork.Service) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.DeleteGitWorktree")
	wt, err := s.GetGitWorktree(ctx, projectID, worktreeID)
	if err != nil {
		return err
	}
	if wt.IsMain {
		return fmt.Errorf("%w: cannot delete main worktree", domain.ErrInvalidInput)
	}
	if err := guardNoRunningTask(ctx, s.db, worktreeID); err != nil {
		return err
	}
	repo, err := s.GetGitRepository(ctx, projectID, wt.RepositoryID)
	if err != nil {
		return err
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	if err := gitSvc.RemoveWorktree(ctx, opened, wt.Path, force); err != nil {
		return mapGitworkRemoveErr(err)
	}
	res := s.db.WithContext(ctx).Delete(&domain.GitWorktree{}, "id = ?", worktreeID)
	if res.Error != nil {
		return fmt.Errorf("delete git worktree row: %w", res.Error)
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapGitworkCreateErr(err error) error {
	switch {
	case errors.Is(err, gitwork.ErrWorktreeExists):
		return domain.NewGitErr(domain.GitCodePathExists, "worktree path already exists")
	case errors.Is(err, gitwork.ErrBranchCheckedOut):
		return domain.NewGitErr(domain.GitCodeBranchCheckedOut, "branch is checked out in another worktree")
	default:
		return err
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapGitworkRemoveErr(err error) error {
	if errors.Is(err, gitwork.ErrDirty) {
		return domain.NewGitErr(domain.GitCodePathExists, "worktree has uncommitted changes; use force")
	}
	return err
}

// ReconcileGitRepository syncs worktree rows with git worktree list output.
func (s *Store) ReconcileGitRepository(ctx context.Context, projectID, repoID string, gitSvc gitwork.Service) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ReconcileGitRepository")
	repo, err := s.GetGitRepository(ctx, projectID, repoID)
	if err != nil {
		return err
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	live, err := gitSvc.ListWorktrees(ctx, opened)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	livePaths := make(map[string]gitwork.Worktree, len(live))
	for _, wt := range live {
		livePaths[filepath.Clean(wt.Path)] = wt
	}
	var dbRows []domain.GitWorktree
	if err := s.db.WithContext(ctx).Where("repository_id = ?", repoID).Find(&dbRows).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbByPath := make(map[string]domain.GitWorktree, len(dbRows))
		for _, row := range dbRows {
			dbByPath[filepath.Clean(row.Path)] = row
		}
		for path, wt := range livePaths {
			if _, ok := dbByPath[path]; ok {
				continue
			}
			name := "discovered-" + worktreeDisplayName(path)
			if err := tx.Create(&domain.GitWorktree{
				ID:           uuid.NewString(),
				RepositoryID: repoID,
				Path:         path,
				Name:         name,
				IsMain:       wt.IsMain,
				CreatedAt:    now,
			}).Error; err != nil {
				return err
			}
		}
		for _, row := range dbRows {
			if row.IsMain {
				continue
			}
			if _, ok := livePaths[filepath.Clean(row.Path)]; ok {
				continue
			}
			ref, err := hasAnyTaskOnWorktree(ctx, tx, row.ID)
			if err != nil {
				return err
			}
			if ref {
				return domain.NewGitErr(domain.GitCodeHasRunningTask, "worktree missing on disk but referenced by tasks")
			}
			if err := tx.Delete(&domain.GitWorktree{}, "id = ?", row.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
