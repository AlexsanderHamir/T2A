package store

import (
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

const storeLogCmd = "taskapi"

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.NewStore")
	return &Store{db: db}
}

type CreateTaskInput struct {
	ID               string
	DraftID          string
	Title            string
	InitialPrompt    string
	Status           domain.Status
	Priority         domain.Priority
	TaskType         domain.TaskType
	ParentID         *string
	ChecklistInherit bool
}

// ParentFieldPatch updates parent_id when non-nil. Clear true means set parent to null.
type ParentFieldPatch struct {
	Clear bool
	ID    string
}

type UpdateTaskInput struct {
	Title            *string
	InitialPrompt    *string
	Status           *domain.Status
	Priority         *domain.Priority
	TaskType         *domain.TaskType
	Parent           *ParentFieldPatch
	ChecklistInherit *bool
}
