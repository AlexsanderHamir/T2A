package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
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
	var rows []model.GitWorktree
	err := s.db.WithContext(ctx).
		Where("repository_id = ?", repoID).
		Order("is_main DESC, created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list git worktrees: %w", err)
	}
	return model.ToDomainGitWorktrees(rows), nil
}

// GetGitWorktree returns one worktree by ID. The projectID parameter is
// accepted for API compatibility but ignored — repositories are global.
func (s *Store) GetGitWorktree(ctx context.Context, projectID, worktreeID string) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetGitWorktree")
	var row model.GitWorktree
	err := s.db.WithContext(ctx).
		Where("id = ?", worktreeID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitWorktree{}, domain.NewGitErr(domain.GitCodeWorktreeNotFound, "worktree not found")
		}
		return domain.GitWorktree{}, fmt.Errorf("get git worktree: %w", err)
	}
	return model.ToDomainGitWorktree(row), nil
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
	if err := guardNoRunningTask(ctx, s.db, worktreeID); err != nil {
		return err
	}
	if !wt.IsMain {
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
	}
	res := s.db.WithContext(ctx).Delete(&model.GitWorktree{}, "id = ?", worktreeID)
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
