package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"gorm.io/gorm"
)

// CreateGitRepositoryInput registers a main git checkout for a project.
type CreateGitRepositoryInput struct {
	Path          string
	HostPath      string
	DefaultBranch string
}

// ListGitRepositories returns all registered repositories ordered by created_at.
// projectID is accepted for API-route compatibility but ignored (repos are global).
func (s *Store) ListGitRepositories(ctx context.Context, projectID string) ([]domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListGitRepositories")
	var rows []model.GitRepository
	err := s.db.WithContext(ctx).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list git repositories: %w", err)
	}
	return model.ToDomainGitRepositories(rows), nil
}

// CountGitRepositories returns the total number of registered git repositories.
func (s *Store) CountGitRepositories(ctx context.Context) (int64, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.CountGitRepositories")
	var n int64
	err := s.db.WithContext(ctx).Model(&model.GitRepository{}).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count git repositories: %w", err)
	}
	return n, nil
}

// GetGitRepository returns one repository by ID.
// projectID is accepted for API-route compatibility but ignored (repos are global).
func (s *Store) GetGitRepository(ctx context.Context, projectID, repoID string) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetGitRepository")
	var row model.GitRepository
	err := s.db.WithContext(ctx).
		Where("id = ?", strings.TrimSpace(repoID)).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found")
		}
		return domain.GitRepository{}, fmt.Errorf("get git repository: %w", err)
	}
	return model.ToDomainGitRepository(row), nil
}

// CreateGitRepository validates path with git, then inserts repository + main worktree + current branch.
// projectID is accepted for API-route compatibility but not stored (repos are global).
func (s *Store) CreateGitRepository(ctx context.Context, projectID string, input CreateGitRepositoryInput, gitSvc gitwork.Service) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.CreateGitRepository")
	repo, err := s.registerGitRepository(ctx, input, gitSvc)
	if err != nil {
		return domain.GitRepository{}, err
	}
	if err := s.seedMainWorktreeWithCurrentBranch(ctx, repo, gitSvc); err != nil {
		return domain.GitRepository{}, err
	}
	return repo, nil
}

// DeleteGitRepository removes a repository when no running tasks reference it.
// projectID is accepted for API-route compatibility but ignored (repos are global).
func (s *Store) DeleteGitRepository(ctx context.Context, projectID, repoID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.DeleteGitRepository")
	if _, err := s.GetGitRepository(ctx, projectID, repoID); err != nil {
		return err
	}
	if err := guardNoRunningTask(ctx, s.db, repoID); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).
		Where("id = ?", repoID).
		Delete(&model.GitRepository{})
	if res.Error != nil {
		return fmt.Errorf("delete git repository: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found")
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func worktreeDisplayName(path string) string {
	base := filepath.Base(filepath.Clean(path))
	if base == "" || base == "." {
		return "worktree"
	}
	return base
}
