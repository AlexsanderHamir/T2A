package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"gorm.io/gorm"
)

// TaskGitContext is the resolved filesystem path and branch name for a task binding.
type TaskGitContext struct {
	WorktreePath string
	BranchName   string
}

// GetGitWorktreeByID loads a worktree row by primary key.
func (s *Store) GetGitWorktreeByID(ctx context.Context, worktreeID string) (domain.GitWorktree, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetGitWorktreeByID")
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		return domain.GitWorktree{}, fmt.Errorf("%w: worktree_id required", domain.ErrInvalidInput)
	}
	var row model.GitWorktree
	err := s.db.WithContext(ctx).Where("id = ?", worktreeID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitWorktree{}, domain.NewGitErr(domain.GitCodeWorktreeNotFound, "worktree not found")
		}
		return domain.GitWorktree{}, fmt.Errorf("get git worktree: %w", err)
	}
	return model.ToDomainGitWorktree(row), nil
}

// GetGitBranchByID loads a branch row by primary key.
func (s *Store) GetGitBranchByID(ctx context.Context, branchID string) (domain.GitBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetGitBranchByID")
	branchID = strings.TrimSpace(branchID)
	if branchID == "" {
		return domain.GitBranch{}, fmt.Errorf("%w: branch_id required", domain.ErrInvalidInput)
	}
	var row model.GitBranch
	err := s.db.WithContext(ctx).Where("id = ?", branchID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitBranch{}, domain.NewGitErr(domain.GitCodeBranchNotFound, "branch not found")
		}
		return domain.GitBranch{}, fmt.Errorf("get git branch: %w", err)
	}
	return model.ToDomainGitBranch(row), nil
}

// GetGitRepositoryByID loads a repository row by primary key.
func (s *Store) GetGitRepositoryByID(ctx context.Context, repoID string) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetGitRepositoryByID")
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return domain.GitRepository{}, fmt.Errorf("%w: repository_id required", domain.ErrInvalidInput)
	}
	var row model.GitRepository
	err := s.db.WithContext(ctx).Where("id = ?", repoID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeRepositoryNotFound, "repository not found")
		}
		return domain.GitRepository{}, fmt.Errorf("get git repository: %w", err)
	}
	return model.ToDomainGitRepository(row), nil
}

// ValidateTaskWorktreeBranchBinding checks worktree_branch_id exists and, when
// projectID is set, that project.repository_id matches the association's repo.
func (s *Store) ValidateTaskWorktreeBranchBinding(ctx context.Context, projectID *string, worktreeBranchID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ValidateTaskWorktreeBranchBinding")
	worktreeBranchID = strings.TrimSpace(worktreeBranchID)
	if worktreeBranchID == "" {
		return fmt.Errorf("%w: worktree_branch_id required", domain.ErrInvalidInput)
	}
	wb, err := s.GetWorktreeBranchByID(ctx, worktreeBranchID)
	if err != nil {
		return err
	}
	wt, err := s.GetGitWorktreeByID(ctx, wb.WorktreeID)
	if err != nil {
		return err
	}
	br, err := s.GetGitBranchByID(ctx, wb.BranchID)
	if err != nil {
		return err
	}
	if wt.RepositoryID != br.RepositoryID {
		return domain.NewGitErr(domain.GitCodeBranchNotAssociated, "worktree and branch belong to different repositories")
	}
	if projectID == nil {
		return nil
	}
	pid := strings.TrimSpace(*projectID)
	if pid == "" {
		return nil
	}
	proj, err := s.GetProject(ctx, pid)
	if err != nil {
		return err
	}
	if proj.RepositoryID == nil || *proj.RepositoryID != wt.RepositoryID {
		return domain.NewGitErr(domain.GitCodeProjectRepoMismatch, "project is not tied to this repository")
	}
	return nil
}

// ResolveTaskGitContextFromAssociation loads worktree path and branch name via worktree_branch_id.
func (s *Store) ResolveTaskGitContextFromAssociation(ctx context.Context, worktreeBranchID string) (TaskGitContext, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ResolveTaskGitContextFromAssociation")
	if err := s.ValidateTaskWorktreeBranchBinding(ctx, nil, worktreeBranchID); err != nil {
		return TaskGitContext{}, err
	}
	wb, err := s.GetWorktreeBranchByID(ctx, worktreeBranchID)
	if err != nil {
		return TaskGitContext{}, err
	}
	wt, err := s.GetGitWorktreeByID(ctx, wb.WorktreeID)
	if err != nil {
		return TaskGitContext{}, err
	}
	br, err := s.GetGitBranchByID(ctx, wb.BranchID)
	if err != nil {
		return TaskGitContext{}, err
	}
	return TaskGitContext{WorktreePath: wt.Path, BranchName: br.Name}, nil
}

// AgentWorkerGitIdle reports whether the worker should stay idle for git registration reasons.
func (s *Store) AgentWorkerGitIdle(ctx context.Context) (idle bool, reason string, err error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.AgentWorkerGitIdle")
	var repoCount int64
	if err := s.db.WithContext(ctx).Model(&model.GitRepository{}).Count(&repoCount).Error; err != nil {
		return false, "", fmt.Errorf("count git repositories: %w", err)
	}
	if repoCount == 0 {
		return true, "no_repository_registered", nil
	}
	var worktrees []model.GitWorktree
	if err := s.db.WithContext(ctx).Find(&worktrees).Error; err != nil {
		return false, "", fmt.Errorf("list git worktrees: %w", err)
	}
	if len(worktrees) == 0 {
		return true, "all_worktrees_invalid", nil
	}
	anyValid := false
	for _, wt := range worktrees {
		st, statErr := os.Stat(wt.Path)
		if statErr == nil && st.IsDir() {
			anyValid = true
			break
		}
	}
	if !anyValid {
		return true, "all_worktrees_invalid", nil
	}
	return false, "", nil
}
