package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/kernel"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BindBranchInput registers or creates a repo-level branch row for worktree assignment.
type BindBranchInput struct {
	Name         string
	CreateBranch bool
	StartPoint   string
}

// ResolveOrCreateBranchForRepo returns a git_branches row for name, creating via git when requested.
func (s *Store) ResolveOrCreateBranchForRepo(
	ctx context.Context,
	repo domain.GitRepository,
	input BindBranchInput,
	gitSvc gitwork.Service,
) (domain.GitBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ResolveOrCreateBranchForRepo")
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.GitBranch{}, fmt.Errorf("%w: branch name required", domain.ErrInvalidInput)
	}
	if input.CreateBranch {
		return s.CreateGitBranchForRepo(ctx, repo.ID, CreateGitBranchInput{
			Name:       name,
			StartPoint: input.StartPoint,
		}, gitSvc)
	}
	var existing model.GitBranch
	err := s.db.WithContext(ctx).
		Where("repository_id = ? AND name = ?", repo.ID, name).
		First(&existing).Error
	if err == nil {
		return model.ToDomainGitBranch(existing), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.GitBranch{}, fmt.Errorf("lookup git branch: %w", err)
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return domain.GitBranch{}, fmt.Errorf("open repository: %w", err)
	}
	live, err := gitSvc.ListBranches(ctx, opened)
	if err != nil {
		return domain.GitBranch{}, fmt.Errorf("list branches: %w", err)
	}
	var headSHA string
	for _, b := range live {
		if b.Name == name {
			headSHA = b.HeadSHA
			break
		}
	}
	if headSHA == "" {
		return domain.GitBranch{}, fmt.Errorf("%w: branch %q not found in repository", domain.ErrInvalidInput, name)
	}
	now := time.Now().UTC()
	row := domain.GitBranch{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Name:         name,
		HeadSHA:      headSHA,
		CreatedAt:    now,
	}
	branchRow := model.FromDomainGitBranch(row)
	if err := s.db.WithContext(ctx).Create(&branchRow).Error; err != nil {
		if kernel.IsDuplicateKey(err) {
			var dup model.GitBranch
			if findErr := s.db.WithContext(ctx).
				Where("repository_id = ? AND name = ?", repo.ID, name).
				First(&dup).Error; findErr == nil {
				return model.ToDomainGitBranch(dup), nil
			}
		}
		return domain.GitBranch{}, fmt.Errorf("register git branch row: %w", err)
	}
	return row, nil
}

// resolveBranchForWorktree resolves or creates a branch and guards one-worktree-per-branch.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Store) resolveBranchForWorktree(
	ctx context.Context,
	repo domain.GitRepository,
	worktreeID string,
	input BindBranchInput,
	gitSvc gitwork.Service,
) (domain.GitBranch, error) {
	br, err := s.ResolveOrCreateBranchForRepo(ctx, repo, input, gitSvc)
	if err != nil {
		return domain.GitBranch{}, err
	}
	if err := s.GuardBranchNotBoundToOtherWorktree(ctx, br.ID, worktreeID); err != nil {
		return domain.GitBranch{}, err
	}
	return br, nil
}
