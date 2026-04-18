package store

import (
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/notify"
	"gorm.io/gorm"
)

const storeLogCmd = "taskapi"

// Store is the public GORM-backed persistence facade for tasks, audit
// events, checklists, drafts, evaluations, cycles/phases, the ready-task
// queue, dev-mirror, and health probes. Behavior is split across
// internal/<domain>/ subpackages; the methods on *Store delegate. See
// pkgs/tasks/store/README.md for the concern map.
type Store struct {
	db     *gorm.DB
	notify notify.Holder
}

// NewStore returns a Store backed by db. The caller still configures
// ready-task notifications via SetReadyTaskNotifier after construction.
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
