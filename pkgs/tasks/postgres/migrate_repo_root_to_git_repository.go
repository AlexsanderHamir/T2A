package postgres

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// migrateRepoRootToGitRepository backfills git_repositories from app_settings.repo_root.
// Idempotent; failures log a warning and do not block startup.
func migrateRepoRootToGitRepository(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.migrateRepoRootToGitRepository")
	var path string
	err := db.WithContext(ctx).
		Raw(`SELECT COALESCE(repo_root, '') FROM app_settings WHERE id = ?`, domain.AppSettingsRowID).
		Scan(&path).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		// Column already dropped on upgraded DBs — nothing to backfill.
		if strings.Contains(strings.ToLower(err.Error()), "repo_root") {
			return nil
		}
		return err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	gitSvc := gitwork.New()
	opened, err := gitSvc.OpenRepository(ctx, path)
	if err != nil {
		slog.Warn("repo_root migration skipped: not a git repository", "path", path, "err", err)
		return nil
	}
	repoRoot := opened.Root
	var existing int64
	if err := db.WithContext(ctx).Model(&domain.GitRepository{}).
		Where("project_id = ? AND path = ?", domain.DefaultProjectID, repoRoot).
		Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	branches, err := gitSvc.ListBranches(ctx, opened)
	if err != nil {
		slog.Warn("repo_root migration skipped: list branches failed", "path", path, "err", err)
		return nil
	}
	now := time.Now().UTC()
	repo := domain.GitRepository{
		ID:            uuid.NewString(),
		ProjectID:     domain.DefaultProjectID,
		Path:          opened.Root,
		DefaultBranch: "main",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	mainWT := domain.GitWorktree{
		ID:           uuid.NewString(),
		RepositoryID: repo.ID,
		Path:         opened.Root,
		Name:         "main",
		IsMain:       true,
		CreatedAt:    now,
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
			Name:         "main",
			CreatedAt:    now,
		})
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&repo).Error; err != nil {
			return err
		}
		if err := tx.Create(&mainWT).Error; err != nil {
			return err
		}
		return tx.Create(&branchRows).Error
	})
}
