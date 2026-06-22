package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateGitRepositoryInput registers a main git checkout for a project.
type CreateGitRepositoryInput struct {
	Path          string
	HostPath      string
	DefaultBranch string
}

// ListGitRepositories returns repositories for a project ordered by created_at.
func (s *Store) ListGitRepositories(ctx context.Context, projectID string) ([]domain.GitRepository, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListGitRepositories")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project_id", domain.ErrInvalidInput)
	}
	var rows []domain.GitRepository
	err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list git repositories: %w", err)
	}
	return rows, nil
}

// GetGitRepository returns one repository scoped to a project.
func (s *Store) GetGitRepository(ctx context.Context, projectID, repoID string) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetGitRepository")
	var row domain.GitRepository
	err := s.db.WithContext(ctx).
		Where("id = ? AND project_id = ?", strings.TrimSpace(repoID), strings.TrimSpace(projectID)).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found")
		}
		return domain.GitRepository{}, fmt.Errorf("get git repository: %w", err)
	}
	return row, nil
}

// CreateGitRepository validates path with git, then inserts repository + main worktree + current branch.
func (s *Store) CreateGitRepository(ctx context.Context, projectID string, input CreateGitRepositoryInput, gitSvc gitwork.Service) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CreateGitRepository")
	projectID = strings.TrimSpace(projectID)
	path := strings.TrimSpace(input.Path)
	if projectID == "" || path == "" {
		return domain.GitRepository{}, fmt.Errorf("%w: project_id and path required", domain.ErrInvalidInput)
	}
	if _, err := s.GetProject(ctx, projectID); err != nil {
		return domain.GitRepository{}, err
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, path)
	if err != nil {
		if errors.Is(err, gitwork.ErrNotARepository) {
			return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeNotARepository, "path is not a git repository")
		}
		return domain.GitRepository{}, fmt.Errorf("open repository: %w", err)
	}
	defaultBranch := strings.TrimSpace(input.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	now := time.Now().UTC()
	repo := domain.GitRepository{
		ID:            uuid.NewString(),
		ProjectID:     projectID,
		Path:          opened.Root,
		HostPath:      strings.TrimSpace(input.HostPath),
		DefaultBranch: defaultBranch,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	mainWT := domain.GitWorktree{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Path:         opened.Root,
		Name:         worktreeDisplayName(opened.Root),
		IsMain:       true,
		CreatedAt:    now,
	}
	branches, err := gitSvc.ListBranches(ctx, opened)
	if err != nil {
		return domain.GitRepository{}, fmt.Errorf("list branches: %w", err)
	}
	var branchRows []domain.GitBranch
	for _, b := range branches {
		branchRows = append(branchRows, domain.GitBranch{
			ID:           uuid.NewString(),
			RepositoryID: repo.ID,
			Name:         b.Name,
			HeadSHA:      b.HeadSHA,
			CreatedAt:    now,
		})
	}
	if len(branchRows) == 0 {
		branchRows = append(branchRows, domain.GitBranch{
			ID:           uuid.NewString(),
			RepositoryID: repo.ID,
			Name:         defaultBranch,
			CreatedAt:    now,
		})
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&repo).Error; err != nil {
			if isDuplicateKey(err) {
				return domain.NewGitErr(domain.GitCodeDuplicate, "repository already registered for this path")
			}
			return err
		}
		if err := tx.Create(&mainWT).Error; err != nil {
			return err
		}
		if len(branchRows) > 0 {
			if err := tx.Create(&branchRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.GitRepository{}, err
	}
	return repo, nil
}

// DeleteGitRepository removes a repository when no running tasks reference it.
func (s *Store) DeleteGitRepository(ctx context.Context, projectID, repoID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteGitRepository")
	if _, err := s.GetGitRepository(ctx, projectID, repoID); err != nil {
		return err
	}
	if err := guardNoRunningTask(ctx, s.db, repoID); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).
		Where("id = ? AND project_id = ?", repoID, projectID).
		Delete(&domain.GitRepository{})
	if res.Error != nil {
		return fmt.Errorf("delete git repository: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found")
	}
	return nil
}

func worktreeDisplayName(path string) string {
	base := filepath.Base(filepath.Clean(path))
	if base == "" || base == "." {
		return "worktree"
	}
	return base
}
