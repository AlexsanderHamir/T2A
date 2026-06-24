package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTreeMigrateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&domain.Project{},
		&domain.GitRepository{},
		&domain.GitWorktree{},
		&domain.GitBranch{},
		&domain.WorktreeBranch{},
		&domain.Task{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedLegacyGitTree creates a pre-ADR-0037 slice: a project owning one repo,
// one worktree, one branch, and a task bound via the legacy two columns.
func seedLegacyGitTree(ctx context.Context, t *testing.T, db *gorm.DB) (wtID, brID, taskID string) {
	t.Helper()
	now := time.Now().UTC()
	proj := domain.DefaultProject(now)
	if err := db.WithContext(ctx).Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	repo := domain.GitRepository{ID: "repo-1", ProjectID: proj.ID, Path: "/repos/app", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}
	if err := db.WithContext(ctx).Create(&repo).Error; err != nil {
		t.Fatal(err)
	}
	wt := domain.GitWorktree{ID: "wt-1", RepositoryID: repo.ID, Path: "/repos/app", Name: "main", IsMain: true, CreatedAt: now}
	if err := db.WithContext(ctx).Create(&wt).Error; err != nil {
		t.Fatal(err)
	}
	br := domain.GitBranch{ID: "br-1", RepositoryID: repo.ID, Name: "main", CreatedAt: now}
	if err := db.WithContext(ctx).Create(&br).Error; err != nil {
		t.Fatal(err)
	}
	task := domain.Task{
		ID:            "task-1",
		Title:         "t",
		InitialPrompt: "p",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		ProjectID:     &proj.ID,
		WorktreeID:    &wt.ID,
		BranchID:      &br.ID,
	}
	if err := db.WithContext(ctx).Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	return wt.ID, br.ID, task.ID
}

func TestMigrateSeedWorktreeBranchTree_backfills(t *testing.T) {
	db := openTreeMigrateDB(t)
	ctx := context.Background()
	wtID, brID, taskID := seedLegacyGitTree(ctx, t, db)

	if err := migrateSeedWorktreeBranchTree(ctx, db); err != nil {
		t.Fatal(err)
	}

	var proj domain.Project
	if err := db.WithContext(ctx).First(&proj, "id = ?", domain.DefaultProjectID).Error; err != nil {
		t.Fatal(err)
	}
	if proj.RepositoryID == nil || *proj.RepositoryID != "repo-1" {
		t.Fatalf("project repository_id=%v want repo-1", proj.RepositoryID)
	}

	var wb domain.WorktreeBranch
	if err := db.WithContext(ctx).First(&wb, "worktree_id = ? AND branch_id = ?", wtID, brID).Error; err != nil {
		t.Fatalf("association not seeded: %v", err)
	}

	var task domain.Task
	if err := db.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.WorktreeBranchID == nil || *task.WorktreeBranchID != wb.ID {
		t.Fatalf("task worktree_branch_id=%v want %s", task.WorktreeBranchID, wb.ID)
	}
}

func TestMigrateSeedWorktreeBranchTree_idempotent(t *testing.T) {
	db := openTreeMigrateDB(t)
	ctx := context.Background()
	wtID, brID, _ := seedLegacyGitTree(ctx, t, db)

	for range 2 {
		if err := migrateSeedWorktreeBranchTree(ctx, db); err != nil {
			t.Fatal(err)
		}
	}

	var n int64
	if err := db.WithContext(ctx).Model(&domain.WorktreeBranch{}).
		Where("worktree_id = ? AND branch_id = ?", wtID, brID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("association count=%d want 1", n)
	}
}

func TestMigrateSeedWorktreeBranchTree_skipsOrphanPairs(t *testing.T) {
	db := openTreeMigrateDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	proj := domain.DefaultProject(now)
	if err := db.WithContext(ctx).Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	// Task references worktree/branch ids that do not exist; no FK rows seeded.
	ghostWT, ghostBR := "missing-wt", "missing-br"
	task := domain.Task{
		ID:            "task-orphan",
		Title:         "t",
		InitialPrompt: "p",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
		ProjectID:     &proj.ID,
		WorktreeID:    &ghostWT,
		BranchID:      &ghostBR,
	}
	if err := db.WithContext(ctx).Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateSeedWorktreeBranchTree(ctx, db); err != nil {
		t.Fatal(err)
	}

	var n int64
	if err := db.WithContext(ctx).Model(&domain.WorktreeBranch{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("association count=%d want 0 (orphan pair skipped)", n)
	}
}
