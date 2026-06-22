package postgres

import (
	"context"
	"os/exec"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	main := initMigrateGitRepo(t)
	settings := domain.DefaultAppSettings()
	settings.RepoRoot = main
	if err := db.Save(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateRepoRootToGitRepository(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&domain.GitRepository{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1", n)
	}
	if err := migrateRepoRootToGitRepository(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domain.GitRepository{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("after second migrate count=%d want 1", n)
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
