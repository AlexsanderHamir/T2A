package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateGitBranchInput creates a local branch via git.
type CreateGitBranchInput struct {
	Name       string
	StartPoint string
}

// ListGitBranches returns branches for a repository.
func (s *Store) ListGitBranches(ctx context.Context, projectID, repoID string) ([]domain.GitBranch, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListGitBranches")
	if _, err := s.GetGitRepository(ctx, projectID, repoID); err != nil {
		return nil, err
	}
	var rows []domain.GitBranch
	err := s.db.WithContext(ctx).
		Where("repository_id = ?", repoID).
		Order("name ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list git branches: %w", err)
	}
	return rows, nil
}

// GetGitBranch returns one branch scoped through its repository project.
func (s *Store) GetGitBranch(ctx context.Context, projectID, branchID string) (domain.GitBranch, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetGitBranch")
	var row domain.GitBranch
	err := s.db.WithContext(ctx).
		Joins("JOIN git_repositories ON git_repositories.id = git_branches.repository_id").
		Where("git_branches.id = ? AND git_repositories.project_id = ?", branchID, projectID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitBranch{}, domain.NewGitErr(domain.GitCodeBranchNotFound, "branch not found")
		}
		return domain.GitBranch{}, fmt.Errorf("get git branch: %w", err)
	}
	return row, nil
}

// CreateGitBranch creates a branch via git and inserts a row.
func (s *Store) CreateGitBranch(ctx context.Context, projectID, repoID string, input CreateGitBranchInput, gitSvc gitwork.Service) (domain.GitBranch, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CreateGitBranch")
	repo, err := s.GetGitRepository(ctx, projectID, repoID)
	if err != nil {
		return domain.GitBranch{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.GitBranch{}, fmt.Errorf("%w: name required", domain.ErrInvalidInput)
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return domain.GitBranch{}, fmt.Errorf("open repository: %w", err)
	}
	created, err := gitSvc.CreateBranch(ctx, opened, name, strings.TrimSpace(input.StartPoint))
	if err != nil {
		if errors.Is(err, gitwork.ErrBranchExists) {
			return domain.GitBranch{}, domain.NewGitErr(domain.GitCodeBranchExists, "branch already exists")
		}
		return domain.GitBranch{}, err
	}
	row := domain.GitBranch{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Name:         created.Name,
		HeadSHA:      created.HeadSHA,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicateKey(err) {
			return domain.GitBranch{}, domain.NewGitErr(domain.GitCodeBranchExists, "branch already exists")
		}
		return domain.GitBranch{}, fmt.Errorf("create git branch row: %w", err)
	}
	return row, nil
}

// DeleteGitBranch removes a branch via git and the database.
func (s *Store) DeleteGitBranch(ctx context.Context, projectID, branchID string, force bool, gitSvc gitwork.Service) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteGitBranch")
	branch, err := s.GetGitBranch(ctx, projectID, branchID)
	if err != nil {
		return err
	}
	if err := guardNoRunningTask(ctx, s.db, branchID); err != nil {
		return err
	}
	repo, err := s.GetGitRepository(ctx, projectID, branch.RepositoryID)
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
	worktrees, err := gitSvc.ListWorktrees(ctx, opened)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	for _, wt := range worktrees {
		if wt.Branch == branch.Name {
			return domain.NewGitErr(domain.GitCodeBranchCheckedOut, "branch is checked out in a worktree")
		}
	}
	if err := gitSvc.DeleteBranch(ctx, opened, branch.Name, force); err != nil {
		if errors.Is(err, gitwork.ErrBranchCheckedOut) {
			return domain.NewGitErr(domain.GitCodeBranchCheckedOut, "branch is checked out in a worktree")
		}
		return err
	}
	res := s.db.WithContext(ctx).Delete(&domain.GitBranch{}, "id = ?", branchID)
	if res.Error != nil {
		return fmt.Errorf("delete git branch row: %w", res.Error)
	}
	return nil
}
