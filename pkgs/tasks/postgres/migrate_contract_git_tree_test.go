package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// legacyGitRepository is the pre-C8 shape with project_id.
type legacyGitRepository struct {
	ID            string    `gorm:"primaryKey"`
	ProjectID     string    `gorm:"not null"`
	Path          string    `gorm:"not null"`
	HostPath      string    `gorm:"not null;default:''"`
	DefaultBranch string    `gorm:"not null;default:main"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (legacyGitRepository) TableName() string { return "git_repositories" }

// legacyTask is the pre-C8 shape with worktree_id and branch_id.
type legacyTask struct {
	ID               string          `gorm:"primaryKey"`
	Title            string          `gorm:"not null"`
	InitialPrompt    string          `gorm:"type:text;not null"`
	Status           domain.Status   `gorm:"not null"`
	Priority         domain.Priority `gorm:"not null"`
	ProjectID        *string         `gorm:"index"`
	Runner           string          `gorm:"not null;default:'cursor'"`
	WorktreeID       *string
	BranchID         *string
	WorktreeBranchID *string
}

func (legacyTask) TableName() string { return "tasks" }

func openContractMigrateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Project{},
		&legacyGitRepository{},
		&model.GitWorktree{},
		&model.GitBranch{},
		&model.WorktreeBranch{},
		&legacyTask{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateContractGitTree_dropsColumns(t *testing.T) {
	db := openContractMigrateDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	proj := model.FromDomainProject(domain.DefaultProject(now))
	if err := db.WithContext(ctx).Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	repo := legacyGitRepository{
		ID: "repo-1", ProjectID: proj.ID, Path: "/repos/app",
		DefaultBranch: "main", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.WithContext(ctx).Create(&repo).Error; err != nil {
		t.Fatal(err)
	}
	wt := model.GitWorktree{ID: "wt-1", RepositoryID: repo.ID, Path: "/repos/app", Name: "main", IsMain: true, CreatedAt: now}
	if err := db.WithContext(ctx).Create(&wt).Error; err != nil {
		t.Fatal(err)
	}
	br := model.GitBranch{ID: "br-1", RepositoryID: repo.ID, Name: "main", CreatedAt: now}
	if err := db.WithContext(ctx).Create(&br).Error; err != nil {
		t.Fatal(err)
	}
	wb := model.WorktreeBranch{ID: "wb-1", WorktreeID: wt.ID, BranchID: br.ID, CreatedAt: now}
	if err := db.WithContext(ctx).Create(&wb).Error; err != nil {
		t.Fatal(err)
	}

	task := legacyTask{
		ID: "task-1", Title: "t", InitialPrompt: "p",
		Status: domain.StatusReady, Priority: domain.PriorityMedium,
		ProjectID: &proj.ID, WorktreeID: &wt.ID, BranchID: &br.ID,
		WorktreeBranchID: &wb.ID,
	}
	if err := db.WithContext(ctx).Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateContractGitTree(ctx, db); err != nil {
		t.Fatal(err)
	}

	if tableHasColumnPortable(db, "git_repositories", "project_id") {
		t.Fatal("git_repositories.project_id not dropped")
	}
	if tableHasColumnPortable(db, "tasks", "worktree_id") {
		t.Fatal("tasks.worktree_id not dropped")
	}
	if tableHasColumnPortable(db, "tasks", "branch_id") {
		t.Fatal("tasks.branch_id not dropped")
	}

	var taskRow struct {
		ProjectID        *string
		WorktreeBranchID *string
	}
	if err := db.WithContext(ctx).Raw(`SELECT project_id, worktree_branch_id FROM tasks WHERE id = ?`, "task-1").Scan(&taskRow).Error; err != nil {
		t.Fatal(err)
	}
	if taskRow.ProjectID != nil {
		t.Fatalf("task project_id=%v want nil (default project nulled)", *taskRow.ProjectID)
	}
	if taskRow.WorktreeBranchID == nil || *taskRow.WorktreeBranchID != wb.ID {
		t.Fatalf("task worktree_branch_id=%v want %s", taskRow.WorktreeBranchID, wb.ID)
	}
}

func TestMigrateContractGitTree_idempotent(t *testing.T) {
	db := openContractMigrateDB(t)
	ctx := context.Background()

	for range 2 {
		if err := migrateContractGitTree(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
}
