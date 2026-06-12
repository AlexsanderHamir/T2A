package checklist

import (
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func TestValidateParentCanHaveSubtasksInTx_requiresCriterionOnRootParent(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := t.Context()

	parent := &domain.Task{
		ID: "parent-1", Title: "p", InitialPrompt: "p", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}
	if err := db.WithContext(ctx).Create(parent).Error; err != nil {
		t.Fatal(err)
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ValidateParentCanHaveSubtasksInTx(tx, parent.ID)
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}

	it := &domain.TaskChecklistItem{ID: "item-1", TaskID: parent.ID, SortOrder: 1, Text: "done when file exists"}
	if err := db.WithContext(ctx).Create(it).Error; err != nil {
		t.Fatal(err)
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ValidateParentCanHaveSubtasksInTx(tx, parent.ID)
	})
	if err != nil {
		t.Fatalf("want nil after criterion added: %v", err)
	}
}

func TestValidateCanMarkDone_allowsDoneWithOpenSubtask(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := t.Context()

	parent := &domain.Task{
		ID: "parent-2", Title: "p", InitialPrompt: "p", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}
	child := &domain.Task{
		ID: "child-1", Title: "c", InitialPrompt: "c", Status: domain.StatusReady, Priority: domain.PriorityMedium,
		ParentID: &parent.ID,
	}
	if err := db.WithContext(ctx).Create(parent).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Create(child).Error; err != nil {
		t.Fatal(err)
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ValidateCanMarkDoneInTx(tx, parent.ID)
	})
	if err != nil {
		t.Fatalf("empty checklist parent with open subtask should pass: %v", err)
	}
}
