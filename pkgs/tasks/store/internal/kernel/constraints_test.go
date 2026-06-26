package kernel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func TestIsDuplicateKey(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"gorm sentinel", gorm.ErrDuplicatedKey, true},
		{"sqlite unique", errors.New("UNIQUE constraint failed: git_branches.id"), true},
		{"postgres unique", errors.New(`ERROR: duplicate key value violates unique constraint "git_branches_pkey" (SQLSTATE 23505)`), true},
		{"random", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDuplicateKey(tt.err); got != tt.want {
				t.Fatalf("got %v want %v for %v", got, tt.want, tt.err)
			}
		})
	}
}

func TestIsDuplicatePrimaryKey(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		tableName string
		want      bool
	}{
		{"nil", nil, "tasks", false},
		{"gorm sentinel", gorm.ErrDuplicatedKey, "tasks", true},
		{"sqlite tasks.id", errors.New("UNIQUE constraint failed: tasks.id"), "tasks", true},
		{"sqlite wrong table", errors.New("UNIQUE constraint failed: other.id"), "tasks", false},
		{"postgres pkey", errors.New(`ERROR: duplicate key value violates unique constraint "tasks_pkey" (SQLSTATE 23505)`), "tasks", true},
		{"postgres other table pkey", errors.New(`ERROR: duplicate key value violates unique constraint "foo_pkey" (SQLSTATE 23505)`), "tasks", false},
		{"random", errors.New("connection refused"), "tasks", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDuplicatePrimaryKey(tt.err, tt.tableName); got != tt.want {
				t.Fatalf("got %v want %v for %v", got, tt.want, tt.err)
			}
		})
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"gorm sentinel", gorm.ErrForeignKeyViolated, true},
		{"sqlite fk", errors.New("FOREIGN KEY constraint failed"), true},
		{"postgres fk", errors.New(`ERROR: insert or update on table "tasks" violates foreign key constraint "fk_tasks_project" (SQLSTATE 23503)`), true},
		{"random", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForeignKeyViolation(tt.err); got != tt.want {
				t.Fatalf("got %v want %v for %v", got, tt.want, tt.err)
			}
		})
	}
}

func TestIsCheckConstraintViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"gorm sentinel", gorm.ErrCheckConstraintViolated, true},
		{"sqlite check", errors.New("CHECK constraint failed: chk_tasks_status"), true},
		{"postgres check", errors.New(`ERROR: new row for relation "tasks" violates check constraint "chk_tasks_status" (SQLSTATE 23514)`), true},
		{"random", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCheckConstraintViolation(tt.err); got != tt.want {
				t.Fatalf("got %v want %v for %v", got, tt.want, tt.err)
			}
		})
	}
}

func TestMapWriteError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		err     error
		wantIs  error
		wantNil bool
	}{
		{"nil", nil, nil, true},
		{"gorm duplicate", gorm.ErrDuplicatedKey, domain.ErrConflict, false},
		{"unique", errors.New("UNIQUE constraint failed: projects.id"), domain.ErrConflict, false},
		{"foreign key", errors.New("foreign key constraint failed"), domain.ErrInvalidInput, false},
		{"check", gorm.ErrCheckConstraintViolated, domain.ErrInvalidInput, false},
		{"other", errors.New("connection refused"), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapWriteError(tt.err, "duplicate project row")
			if tt.wantNil {
				if got != nil {
					t.Fatalf("got %v want nil", got)
				}
				return
			}
			if tt.wantIs != nil && !errors.Is(got, tt.wantIs) {
				t.Fatalf("got %v want errors.Is(..., %v)", got, tt.wantIs)
			}
		})
	}
}

func TestConstraintClassifiers_sqlite(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("duplicate key", func(t *testing.T) {
		id := "kernel-dup-project"
		row := domain.Project{
			ID:        id,
			Name:      "dup",
			Status:    domain.ProjectStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
		err := db.WithContext(ctx).Create(&row).Error
		if err == nil {
			t.Fatal("expected duplicate insert error")
		}
		if !IsDuplicateKey(err) {
			t.Fatalf("IsDuplicateKey(%v) = false", err)
		}
	})

	t.Run("foreign key", func(t *testing.T) {
		badProject := "missing-project-id"
		task := domain.Task{
			ID:            "kernel-fk-task",
			Title:         "fk",
			InitialPrompt: "x",
			Status:        domain.StatusReady,
			Priority:      domain.PriorityMedium,
			ProjectID:     &badProject,
			Runner:        "cursor",
		}
		err := db.WithContext(ctx).Create(&task).Error
		if err == nil {
			t.Fatal("expected foreign key error")
		}
		if !IsForeignKeyViolation(err) {
			t.Fatalf("IsForeignKeyViolation(%v) = false", err)
		}
	})

	t.Run("check constraint", func(t *testing.T) {
		task := domain.Task{
			ID:            "kernel-check-task",
			Title:         "check",
			InitialPrompt: "x",
			Status:        domain.Status("not-a-status"),
			Priority:      domain.PriorityMedium,
			Runner:        "cursor",
		}
		err := db.WithContext(ctx).Create(&task).Error
		if err == nil {
			t.Fatal("expected check constraint error")
		}
		if !IsCheckConstraintViolation(err) {
			t.Fatalf("IsCheckConstraintViolation(%v) = false", err)
		}
	})
}
