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

// registerGitRepository resolves git identity and inserts a repository row.
// Duplicate detection uses git_common_dir, not path.
func (s *Store) registerGitRepository(ctx context.Context, input CreateGitRepositoryInput, gitSvc gitwork.Service) (domain.GitRepository, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.registerGitRepository")
	path := strings.TrimSpace(input.Path)
	if path == "" {
		return domain.GitRepository{}, fmt.Errorf("%w: path required", domain.ErrInvalidInput)
	}
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	mainRoot, commonDir, err := gitSvc.ResolveRegistration(ctx, path)
	if err != nil {
		if errors.Is(err, gitwork.ErrNotARepository) {
			return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeNotARepository, "path is not a git repository")
		}
		return domain.GitRepository{}, fmt.Errorf("resolve repository: %w", err)
	}
	var existing int64
	if err := s.db.WithContext(ctx).Model(&model.GitRepository{}).
		Where("git_common_dir = ?", commonDir).
		Count(&existing).Error; err != nil {
		return domain.GitRepository{}, err
	}
	if existing > 0 {
		return domain.GitRepository{}, domain.NewGitErr(domain.GitCodeDuplicate, "repository already registered")
	}
	defaultBranch := strings.TrimSpace(input.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	now := time.Now().UTC()
	repo := domain.GitRepository{
		ID:            uuid.NewString(),
		Path:          mainRoot,
		GitCommonDir:  commonDir,
		HostPath:      strings.TrimSpace(input.HostPath),
		DefaultBranch: defaultBranch,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repoRow := model.FromDomainGitRepository(repo)
		if err := tx.Create(&repoRow).Error; err != nil {
			if kernel.IsDuplicateKey(err) {
				return domain.NewGitErr(domain.GitCodeDuplicate, "repository already registered")
			}
			return err
		}
		return nil
	})
	if err != nil {
		return domain.GitRepository{}, err
	}
	return repo, nil
}

// seedMainWorktreeWithCurrentBranch inserts the main worktree row and one branch
// row for the checkout branch currently at the main root.
func (s *Store) seedMainWorktreeWithCurrentBranch(ctx context.Context, repo domain.GitRepository, gitSvc gitwork.Service) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.seedMainWorktreeWithCurrentBranch")
	if gitSvc == nil {
		gitSvc = gitwork.New()
	}
	opened, err := gitSvc.OpenRepository(ctx, repo.Path)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	branches, err := gitSvc.ListBranches(ctx, opened)
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}
	branchName := strings.TrimSpace(repo.DefaultBranch)
	if branchName == "" {
		branchName = "main"
	}
	headSHA := ""
	for _, b := range branches {
		if b.IsCurrent && strings.TrimSpace(b.Name) != "" {
			branchName = b.Name
			headSHA = b.HeadSHA
			break
		}
	}
	now := time.Now().UTC()
	branchRow := domain.GitBranch{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Name:         branchName,
		HeadSHA:      headSHA,
		CreatedAt:    now,
	}
	mainWT := domain.GitWorktree{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Path:         repo.Path,
		Name:         worktreeDisplayName(repo.Path),
		IsMain:       true,
		BranchID:     branchRow.ID,
		CreatedAt:    now,
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		branchModel := model.FromDomainGitBranch(branchRow)
		if err := tx.Create(&branchModel).Error; err != nil {
			return err
		}
		mainWTModel := model.FromDomainGitWorktree(mainWT)
		if err := tx.Create(&mainWTModel).Error; err != nil {
			return err
		}
		return nil
	})
}
