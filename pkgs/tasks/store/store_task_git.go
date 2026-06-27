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
	WorktreeID   string
	BranchID     string
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

// ValidateTaskWorktreeBinding checks worktree_id exists and, when projectID is
// set, that project.repository_id matches the worktree's repo.
func (s *Store) ValidateTaskWorktreeBinding(ctx context.Context, projectID *string, worktreeID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ValidateTaskWorktreeBinding")
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		return fmt.Errorf("%w: worktree_id required", domain.ErrInvalidInput)
	}
	wt, err := s.GetGitWorktreeByID(ctx, worktreeID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(wt.BranchID) == "" {
		return fmt.Errorf("%w: worktree has no branch assigned", domain.ErrInvalidInput)
	}
	if _, err := s.GetGitBranchByID(ctx, wt.BranchID); err != nil {
		return err
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

// ResolveTaskGitContext loads worktree path and branch name via worktree_id.
func (s *Store) ResolveTaskGitContext(ctx context.Context, worktreeID string) (TaskGitContext, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ResolveTaskGitContext")
	if err := s.ValidateTaskWorktreeBinding(ctx, nil, worktreeID); err != nil {
		return TaskGitContext{}, err
	}
	wt, err := s.GetGitWorktreeByID(ctx, worktreeID)
	if err != nil {
		return TaskGitContext{}, err
	}
	br, err := s.GetGitBranchByID(ctx, wt.BranchID)
	if err != nil {
		return TaskGitContext{}, err
	}
	return TaskGitContext{
		WorktreeID:   wt.ID,
		BranchID:     br.ID,
		WorktreePath: wt.Path,
		BranchName:   br.Name,
	}, nil
}

// GuardBranchNotBoundToOtherWorktree rejects when branchID is already assigned to another worktree.
func (s *Store) GuardBranchNotBoundToOtherWorktree(ctx context.Context, branchID, exceptWorktreeID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GuardBranchNotBoundToOtherWorktree")
	branchID = strings.TrimSpace(branchID)
	if branchID == "" {
		return fmt.Errorf("%w: branch_id required", domain.ErrInvalidInput)
	}
	var other model.GitWorktree
	q := s.db.WithContext(ctx).Where("branch_id = ?", branchID)
	if exceptWorktreeID = strings.TrimSpace(exceptWorktreeID); exceptWorktreeID != "" {
		q = q.Where("id <> ?", exceptWorktreeID)
	}
	err := q.First(&other).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check branch worktree binding: %w", err)
	}
	return domain.NewGitErr(domain.GitCodeBranchBoundToWorktree, "branch is already assigned to another worktree")
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
