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

// legacyRev3GitWorktree is the rev-3 git_worktrees shape (no branch_id).
type legacyRev3GitWorktree struct {
	ID             string    `gorm:"column:id;primaryKey"`
	RepositoryID   string    `gorm:"column:repository_id;not null"`
	Path           string    `gorm:"column:path;not null"`
	Name           string    `gorm:"column:name;not null"`
	IsMain         bool      `gorm:"column:is_main;not null;default:false"`
	ActiveBranchID *string   `gorm:"column:active_branch_id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
}

func (legacyRev3GitWorktree) TableName() string { return "git_worktrees" }

// legacyRev3Task is the rev-3 tasks shape with worktree_branch_id only.
type legacyRev3Task struct {
	ID               string  `gorm:"column:id;primaryKey"`
	Title            string  `gorm:"column:title;not null"`
	InitialPrompt    string  `gorm:"column:initial_prompt;not null"`
	Status           string  `gorm:"column:status;not null;default:ready"`
	Priority         string  `gorm:"column:priority;not null;default:medium"`
	WorktreeBranchID *string `gorm:"column:worktree_branch_id"`
}

func (legacyRev3Task) TableName() string { return "tasks" }

func openRev3FixedBranchDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&domain.GitRepository{},
		&legacyRev3GitWorktree{},
		&domain.GitBranch{},
		&testWorktreeBranch{},
		&legacyRev3Task{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateExpandFixedWorktreeBranch_backfillsBeforeAutoMigrate(t *testing.T) {
	db := openRev3FixedBranchDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	repo := domain.GitRepository{ID: "repo-1", Path: "/repos/app", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}
	if err := db.WithContext(ctx).Create(&repo).Error; err != nil {
		t.Fatal(err)
	}
	br := domain.GitBranch{ID: "br-1", RepositoryID: repo.ID, Name: "main", CreatedAt: now}
	if err := db.WithContext(ctx).Create(&br).Error; err != nil {
		t.Fatal(err)
	}
	active := br.ID
	wt := legacyRev3GitWorktree{
		ID: "wt-1", RepositoryID: repo.ID, Path: "/repos/app", Name: "main",
		IsMain: true, ActiveBranchID: &active, CreatedAt: now,
	}
	if err := db.WithContext(ctx).Create(&wt).Error; err != nil {
		t.Fatal(err)
	}
	wb := testWorktreeBranch{ID: "wb-1", WorktreeID: wt.ID, BranchID: br.ID, CreatedAt: now}
	if err := db.WithContext(ctx).Create(&wb).Error; err != nil {
		t.Fatal(err)
	}
	task := legacyRev3Task{
		ID: "task-1", Title: "t", InitialPrompt: "p",
		Status: string(domain.StatusReady), Priority: string(domain.PriorityMedium),
		WorktreeBranchID: &wb.ID,
	}
	if err := db.WithContext(ctx).Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateExpandFixedWorktreeBranch(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).AutoMigrate(&domain.GitWorktree{}, &domain.Task{}); err != nil {
		t.Fatalf("automigrate after expand: %v", err)
	}
	if err := migrateFixedWorktreeBranch(ctx, db); err != nil {
		t.Fatal(err)
	}

	var got domain.GitWorktree
	if err := db.WithContext(ctx).First(&got, "id = ?", wt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.BranchID != br.ID {
		t.Fatalf("worktree branch_id=%q want %q", got.BranchID, br.ID)
	}

	var taskRow struct {
		WorktreeID string `gorm:"column:worktree_id"`
	}
	if err := db.WithContext(ctx).Table("tasks").Where("id = ?", task.ID).First(&taskRow).Error; err != nil {
		t.Fatal(err)
	}
	if taskRow.WorktreeID != wt.ID {
		t.Fatalf("task worktree_id=%q want %q", taskRow.WorktreeID, wt.ID)
	}
	if db.Migrator().HasTable("worktree_branches") {
		t.Fatal("worktree_branches table should be dropped")
	}
}
