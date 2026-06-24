package store

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssociateWorktreeBranchInput links a repo-level branch to a worktree directory.
type AssociateWorktreeBranchInput struct {
	WorktreeID string
	BranchID   string
}

// GetWorktreeBranchByID loads a worktree_branches row by primary key.
func (s *Store) GetWorktreeBranchByID(ctx context.Context, id string) (domain.WorktreeBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.GetWorktreeBranchByID")
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.WorktreeBranch{}, fmt.Errorf("%w: worktree_branch_id required", domain.ErrInvalidInput)
	}
	var row domain.WorktreeBranch
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorktreeBranch{}, domain.NewGitErr(domain.GitCodeBranchNotAssociated, "worktree branch association not found")
		}
		return domain.WorktreeBranch{}, fmt.Errorf("get worktree branch: %w", err)
	}
	return row, nil
}

// ListWorktreeBranches returns associations for a worktree ordered by created_at.
func (s *Store) ListWorktreeBranches(ctx context.Context, worktreeID string) ([]domain.WorktreeBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ListWorktreeBranches")
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		return nil, fmt.Errorf("%w: worktree_id required", domain.ErrInvalidInput)
	}
	var rows []domain.WorktreeBranch
	err := s.db.WithContext(ctx).
		Where("worktree_id = ?", worktreeID).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list worktree branches: %w", err)
	}
	return rows, nil
}

// AssociateWorktreeBranch creates a worktree↔branch association after validating
// both endpoints belong to the same repository.
func (s *Store) AssociateWorktreeBranch(ctx context.Context, input AssociateWorktreeBranchInput) (domain.WorktreeBranch, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.AssociateWorktreeBranch")
	wt, err := s.GetGitWorktreeByID(ctx, input.WorktreeID)
	if err != nil {
		return domain.WorktreeBranch{}, err
	}
	br, err := s.GetGitBranchByID(ctx, input.BranchID)
	if err != nil {
		return domain.WorktreeBranch{}, err
	}
	if wt.RepositoryID != br.RepositoryID {
		return domain.WorktreeBranch{}, fmt.Errorf("%w: branch_repository_mismatch", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	row := domain.WorktreeBranch{
		ID:         uuid.NewString(),
		WorktreeID: wt.ID,
		BranchID:   br.ID,
		CreatedAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicateKey(err) {
			var existing domain.WorktreeBranch
			if findErr := s.db.WithContext(ctx).
				Where("worktree_id = ? AND branch_id = ?", wt.ID, br.ID).
				First(&existing).Error; findErr == nil {
				return existing, nil
			}
			return domain.WorktreeBranch{}, domain.NewGitErr(domain.GitCodeDuplicate, "branch already associated with worktree")
		}
		return domain.WorktreeBranch{}, fmt.Errorf("associate worktree branch: %w", err)
	}
	return row, nil
}

// RemoveWorktreeBranch deletes an association when no running task uses it.
func (s *Store) RemoveWorktreeBranch(ctx context.Context, worktreeID, branchID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.RemoveWorktreeBranch")
	worktreeID = strings.TrimSpace(worktreeID)
	branchID = strings.TrimSpace(branchID)
	if worktreeID == "" || branchID == "" {
		return fmt.Errorf("%w: worktree_id and branch_id required", domain.ErrInvalidInput)
	}
	var row domain.WorktreeBranch
	if err := s.db.WithContext(ctx).
		Where("worktree_id = ? AND branch_id = ?", worktreeID, branchID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.NewGitErr(domain.GitCodeBranchNotAssociated, "worktree branch association not found")
		}
		return fmt.Errorf("find worktree branch: %w", err)
	}
	if err := guardNoRunningTaskOnWorktreeBranch(ctx, s.db, row.ID); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Delete(&domain.WorktreeBranch{}, "id = ?", row.ID)
	if res.Error != nil {
		return fmt.Errorf("remove worktree branch: %w", res.Error)
	}
	return nil
}

// SetActiveBranch marks branchID as the active checkout in worktreeID. Returns
// branch_active_elsewhere when the branch is already active in another worktree.
func (s *Store) SetActiveBranch(ctx context.Context, worktreeID, branchID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.SetActiveBranch")
	worktreeID = strings.TrimSpace(worktreeID)
	branchID = strings.TrimSpace(branchID)
	if worktreeID == "" || branchID == "" {
		return fmt.Errorf("%w: worktree_id and branch_id required", domain.ErrInvalidInput)
	}
	if err := s.guardBranchNotActiveElsewhere(ctx, worktreeID, branchID); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Model(&domain.GitWorktree{}).
		Where("id = ?", worktreeID).
		Update("active_branch_id", branchID)
	if res.Error != nil {
		return fmt.Errorf("set active branch: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewGitErr(domain.GitCodeWorktreeNotFound, "worktree not found")
	}
	return nil
}

// ClearActiveBranch clears active_branch_id on a worktree when it matches branchID.
func (s *Store) ClearActiveBranch(ctx context.Context, worktreeID, branchID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.ClearActiveBranch")
	worktreeID = strings.TrimSpace(worktreeID)
	branchID = strings.TrimSpace(branchID)
	if worktreeID == "" {
		return fmt.Errorf("%w: worktree_id required", domain.ErrInvalidInput)
	}
	q := s.db.WithContext(ctx).Model(&domain.GitWorktree{}).Where("id = ?", worktreeID)
	if branchID != "" {
		q = q.Where("active_branch_id = ?", branchID)
	}
	res := q.Update("active_branch_id", nil)
	if res.Error != nil {
		return fmt.Errorf("clear active branch: %w", res.Error)
	}
	return nil
}

// GuardBranchNotActiveElsewhere rejects binding when branchID is the active checkout in another worktree.
func (s *Store) GuardBranchNotActiveElsewhere(ctx context.Context, worktreeID, branchID string) error {
	return s.guardBranchNotActiveElsewhere(ctx, worktreeID, branchID)
}

func (s *Store) guardBranchNotActiveElsewhere(ctx context.Context, worktreeID, branchID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.guardBranchNotActiveElsewhere")
	var other domain.GitWorktree
	err := s.db.WithContext(ctx).
		Where("active_branch_id = ? AND id <> ?", branchID, worktreeID).
		First(&other).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check active branch: %w", err)
	}
	return domain.NewGitErr(domain.GitCodeBranchActiveElsewhere, "branch is the active checkout in another worktree")
}
