package postgres

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func TestMigrateRepoRootToGitRepository_idempotentWhenRepoAlreadyRegistered(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	seedLegacyAppSettingsWithRepoRoot(ctx, t, db)
	main := initMigrateGitRepo(t)

	gitSvc := gitwork.New()
	opened, err := gitSvc.OpenRepository(ctx, main)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	existing := model.GitRepository{
		ID:            "existing-repo",
		Path:          opened.Root,
		DefaultBranch: "main",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.WithContext(ctx).Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	// Legacy repo_root may differ from the canonical path stored on git_repositories.
	if err := db.WithContext(ctx).Exec(
		`UPDATE app_settings SET repo_root = ? WHERE id = ?`, main+`\`, domain.AppSettingsRowID,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateRepoRootToGitRepository(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&model.GitRepository{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1", n)
	}
}

func TestMigrateRepoRootToGitRepository_idempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	seedLegacyAppSettingsWithRepoRoot(ctx, t, db)
	main := initMigrateGitRepo(t)
	if err := db.WithContext(ctx).Exec(
		`UPDATE app_settings SET repo_root = ? WHERE id = ?`, main, domain.AppSettingsRowID,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateRepoRootToGitRepository(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&model.GitRepository{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1", n)
	}
	if err := migrateRepoRootToGitRepository(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.GitRepository{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("after second migrate count=%d want 1", n)
	}
}

// seedLegacyAppSettingsWithRepoRoot creates a pre-ADR-0033 schema slice: app_settings
// still carries repo_root before migrateRepoRootToGitRepository backfills git_repositories.
func seedLegacyAppSettingsWithRepoRoot(ctx context.Context, t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.WithContext(ctx).AutoMigrate(
		&model.Project{},
		&model.AppSettings{},
		&model.GitRepository{},
		&model.GitWorktree{},
		&model.GitBranch{},
	); err != nil {
		t.Fatal(err)
	}
	defaultProject := model.FromDomainProject(domain.DefaultProject(time.Now().UTC()))
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&defaultProject).Error; err != nil {
		t.Fatal(err)
	}
	has, err := tableHasColumn(ctx, db, "app_settings", "repo_root")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		if err := db.WithContext(ctx).Exec(`ALTER TABLE app_settings ADD COLUMN repo_root TEXT NOT NULL DEFAULT ''`).Error; err != nil {
			t.Fatal(err)
		}
	}
	settings := model.FromDomainAppSettings(domain.DefaultAppSettings())
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&settings).Error; err != nil {
		t.Fatal(err)
	}
}

func initMigrateGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runMigrateGit(t, dir, "init", "-b", "main")
	runMigrateGit(t, dir, "config", "user.email", "t@example.com")
	runMigrateGit(t, dir, "config", "user.name", "Test")
	runMigrateGit(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

func runMigrateGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	all := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", all...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
