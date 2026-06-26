package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
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
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/kernel"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListAllGitRepositories returns every registered repository ordered by created_at.
func (s *Store) ListAllGitRepositories(ctx context.Context) ([]domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListAllGitRepositories")
	var rows []model.GitRepository
	err := s.db.WithContext(ctx).Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list all git repositories: %w", err)
	}
	return model.ToDomainGitRepositories(rows), nil
}

// CreateGlobalGitRepository registers a main checkout without project scoping.
func (s *Store) CreateGlobalGitRepository(ctx context.Context, input CreateGitRepositoryInput, gitSvc gitwork.Service) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.CreateGlobalGitRepository")
	path := strings.TrimSpace(input.Path)
	if path == "" {
		return domain.GitRepository{}, fmt.Errorf("%w: path required", domain.ErrInvalidInput)
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
	var existing int64
	if err := s.db.WithContext(ctx).Model(&model.GitRepository{}).
		Where("path = ?", opened.Root).
		Count(&existing).Error; err != nil {
		return domain.GitRepository{}, err
	}
	if existing > 0 {
		return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeDuplicate, "repository already registered for this path")
	}
	defaultBranch := strings.TrimSpace(input.DefaultBranch)
	if defaultBranch == "" {
		branches, listErr := gitSvc.ListBranches(ctx, opened)
		if listErr == nil {
			for _, b := range branches {
				if b.IsCurrent && strings.TrimSpace(b.Name) != "" {
					defaultBranch = b.Name
					break
				}
			}
		}
		if defaultBranch == "" {
			defaultBranch = "main"
		}
	}
	now := time.Now().UTC()
	repo := domain.GitRepository{
		ID:            uuid.NewString(),
		Path:          opened.Root,
		HostPath:      strings.TrimSpace(input.HostPath),
		DefaultBranch: defaultBranch,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return repo, s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repoRow := model.FromDomainGitRepository(repo)
		if err := tx.Create(&repoRow).Error; err != nil {
			if kernel.IsDuplicateKey(err) {
				return domain.NewGitErr(domain.GitCodeDuplicate, "repository already registered for this path")
			}
			return err
		}
		return nil
	})
}

// DeleteGlobalGitRepository removes a repository by id when no running tasks reference it.
func (s *Store) DeleteGlobalGitRepository(ctx context.Context, repoID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.DeleteGlobalGitRepository")
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return fmt.Errorf("%w: repository_id required", domain.ErrInvalidInput)
	}
	if _, err := s.GetGitRepositoryByID(ctx, repoID); err != nil {
		return err
	}
	if err := guardNoRunningTask(ctx, s.db, repoID); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Delete(&model.GitRepository{}, "id = ?", repoID)
	if res.Error != nil {
		return fmt.Errorf("delete git repository: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found")
	}
	return nil
}

// ListGitWorktreesByRepo returns worktrees for a repository (no project scope).
func (s *Store) ListGitWorktreesByRepo(ctx context.Context, repoID string) ([]domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListGitWorktreesByRepo")
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return nil, fmt.Errorf("%w: repository_id required", domain.ErrInvalidInput)
	}
	if _, err := s.GetGitRepositoryByID(ctx, repoID); err != nil {
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

// CreateGitWorktreeForRepo adds a linked worktree via git under a repository.
func (s *Store) CreateGitWorktreeForRepo(ctx context.Context, repoID string, input CreateGitWorktreeInput, gitSvc gitwork.Service) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.CreateGitWorktreeForRepo")
	repo, err := s.GetGitRepositoryByID(ctx, repoID)
	if err != nil {
		return domain.GitWorktree{}, err
	}
	return s.createGitWorktreeOnRepo(ctx, repo, input, gitSvc)
}

// RegisterExistingGitWorktree validates path is a linked worktree of repo, inserts a row,
// and optionally binds a branch association in the same flow.
func (s *Store) RegisterExistingGitWorktree(
	ctx context.Context,
	repoID string,
	path, name string,
	bind BindBranchInput,
	gitSvc gitwork.Service,
) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.RegisterExistingGitWorktree")
	repo, err := s.GetGitRepositoryByID(ctx, repoID)
	if err != nil {
		return domain.GitWorktree{}, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return domain.GitWorktree{}, fmt.Errorf("%w: path required", domain.ErrInvalidInput)
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return domain.GitWorktree{}, fmt.Errorf("open repository: %w", err)
	}
	live, err := gitSvc.ListWorktrees(ctx, opened)
	if err != nil {
		return domain.GitWorktree{}, fmt.Errorf("list worktrees: %w", err)
	}
	cleanPath := filepath.Clean(path)
	var found *gitwork.Worktree
	for i := range live {
		if filepath.Clean(live[i].Path) == cleanPath {
			found = &live[i]
			break
		}
	}
	if found == nil {
		return domain.GitWorktree{}, fmt.Errorf("%w: path is not a linked worktree of this repository", domain.ErrInvalidInput)
	}
	label := strings.TrimSpace(name)
	if label == "" {
		label = worktreeDisplayName(cleanPath)
	}
	now := time.Now().UTC()
	row := domain.GitWorktree{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Path:         cleanPath,
		Name:         label,
		IsMain:       found.IsMain,
		CreatedAt:    now,
	}
	wtRow := model.FromDomainGitWorktree(row)
	if err := s.db.WithContext(ctx).Create(&wtRow).Error; err != nil {
		if kernel.IsDuplicateKey(err) {
			return domain.GitWorktree{}, domain.NewGitErr(domain.GitCodePathExists, "worktree path already registered")
		}
		return domain.GitWorktree{}, fmt.Errorf("register git worktree: %w", err)
	}
	bindName := strings.TrimSpace(bind.Name)
	if bindName == "" {
		bindName = strings.TrimSpace(found.Branch)
	}
	if bindName != "" {
		if _, err := s.bindWorktreeBranch(ctx, repo, row.ID, BindBranchInput{
			Name:         bindName,
			CreateBranch: bind.CreateBranch,
			StartPoint:   bind.StartPoint,
		}, gitSvc); err != nil {
			_ = s.db.WithContext(ctx).Delete(&model.GitWorktree{}, "id = ?", row.ID)
			return domain.GitWorktree{}, err
		}
	}
	return row, nil
}

// DeleteGitWorktreeByID removes a worktree from disk and the database (no project scope).
func (s *Store) DeleteGitWorktreeByID(ctx context.Context, worktreeID string, force bool, gitSvc gitwork.Service) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.DeleteGitWorktreeByID")
	wt, err := s.GetGitWorktreeByID(ctx, worktreeID)
	if err != nil {
		return err
	}
	if wt.IsMain {
		return fmt.Errorf("%w: cannot delete main worktree", domain.ErrInvalidInput)
	}
	if err := guardNoRunningTask(ctx, s.db, worktreeID); err != nil {
		return err
	}
	repo, err := s.GetGitRepositoryByID(ctx, wt.RepositoryID)
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
	res := s.db.WithContext(ctx).Delete(&model.GitWorktree{}, "id = ?", worktreeID)
	if res.Error != nil {
		return fmt.Errorf("delete git worktree row: %w", res.Error)
	}
	return nil
}

// ListGitBranchesByRepo returns branches for a repository (no project scope).
func (s *Store) ListGitBranchesByRepo(ctx context.Context, repoID string) ([]domain.GitBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListGitBranchesByRepo")
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return nil, fmt.Errorf("%w: repository_id required", domain.ErrInvalidInput)
	}
	if _, err := s.GetGitRepositoryByID(ctx, repoID); err != nil {
		return nil, err
	}
	var rows []model.GitBranch
	err := s.db.WithContext(ctx).
		Where("repository_id = ?", repoID).
		Order("name ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list git branches: %w", err)
	}
	return model.ToDomainGitBranches(rows), nil
}

// CreateGitBranchForRepo creates a branch via git under a repository (no project scope).
func (s *Store) CreateGitBranchForRepo(ctx context.Context, repoID string, input CreateGitBranchInput, gitSvc gitwork.Service) (domain.GitBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.CreateGitBranchForRepo")
	repo, err := s.GetGitRepositoryByID(ctx, repoID)
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
	branchRow := model.FromDomainGitBranch(row)
	if err := s.db.WithContext(ctx).Create(&branchRow).Error; err != nil {
		if kernel.IsDuplicateKey(err) {
			return domain.GitBranch{}, domain.NewGitErr(domain.GitCodeBranchExists, "branch already exists")
		}
		return domain.GitBranch{}, fmt.Errorf("create git branch row: %w", err)
	}
	return row, nil
}

// ListProjectsByRepository returns projects tied to a repository.
func (s *Store) ListProjectsByRepository(ctx context.Context, repoID string) ([]domain.Project, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListProjectsByRepository")
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return nil, fmt.Errorf("%w: repository_id required", domain.ErrInvalidInput)
	}
	if _, err := s.GetGitRepositoryByID(ctx, repoID); err != nil {
		return nil, err
	}
	var rows []model.Project
	err := s.db.WithContext(ctx).
		Where("repository_id = ?", repoID).
		Order("updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list projects by repository: %w", err)
	}
	return model.ToDomainProjects(rows), nil
}

//funclogmeasure:skip category=hot-path reason="Internal helper; trace emitted by calling chokepoint."
func (s *Store) createGitWorktreeOnRepo(ctx context.Context, repo domain.GitRepository, input CreateGitWorktreeInput, gitSvc gitwork.Service) (domain.GitWorktree, error) {
	path := strings.TrimSpace(input.Path)
	branch := strings.TrimSpace(input.Branch)
	if path == "" || branch == "" {
		return domain.GitWorktree{}, fmt.Errorf("%w: path and branch required", domain.ErrInvalidInput)
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return domain.GitWorktree{}, fmt.Errorf("open repository: %w", err)
	}
	wt, err := gitSvc.AddWorktree(ctx, opened, path, gitwork.AddWorktreeOptions{
		Branch:       branch,
		CreateBranch: input.CreateBranch,
		StartPoint:   strings.TrimSpace(input.StartPoint),
	})
	if err != nil {
		return domain.GitWorktree{}, mapGitworkCreateErr(err)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = worktreeDisplayName(wt.Path)
	}
	now := time.Now().UTC()
	row := domain.GitWorktree{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Path:         wt.Path,
		Name:         name,
		IsMain:       false,
		CreatedAt:    now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wtRow := model.FromDomainGitWorktree(row)
		if err := tx.Create(&wtRow).Error; err != nil {
			if kernel.IsDuplicateKey(err) {
				return domain.NewGitErr(domain.GitCodePathExists, "worktree path already registered")
			}
			return err
		}
		return nil
	})
	if err != nil {
		_ = gitSvc.RemoveWorktree(ctx, opened, wt.Path, true)
		return domain.GitWorktree{}, err
	}
	if _, err := s.bindWorktreeBranch(ctx, repo, row.ID, BindBranchInput{
		Name:         branch,
		CreateBranch: false,
		StartPoint:   strings.TrimSpace(input.StartPoint),
	}, gitSvc); err != nil {
		_ = gitSvc.RemoveWorktree(ctx, opened, wt.Path, true)
		_ = s.db.WithContext(ctx).Delete(&model.GitWorktree{}, "id = ?", row.ID)
		return domain.GitWorktree{}, err
	}
	return row, nil
}
