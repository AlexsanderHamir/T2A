package tasks

import (
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// CreateInput is the task creation payload. Re-aliased by the public
// store facade as store.CreateTaskInput so handler code stays
// unchanged.
type CreateInput struct {
	ID               string
	DraftID          string
	Title            string
	InitialPrompt    string
	Status           domain.Status
	Priority         domain.Priority
	TaskType         domain.TaskType
	ParentID         *string
	ChecklistInherit bool
	Runner           string
	CursorModel      string
	// PickupNotBefore is optional; when set, the agent queue excludes this task
	// until the instant has passed (UTC).
	PickupNotBefore *time.Time
}

// ParentFieldPatch updates parent_id when non-nil. Clear true means
// set parent to null. Re-aliased by the public store facade as
// store.ParentFieldPatch.
type ParentFieldPatch struct {
	Clear bool
	ID    string
}

// UpdateInput is the task patch payload. Each pointer field is
// optional; a nil pointer means "do not change". Re-aliased by the
// public store facade as store.UpdateTaskInput.
type UpdateInput struct {
	Title            *string
	InitialPrompt    *string
	Status           *domain.Status
	Priority         *domain.Priority
	TaskType         *domain.TaskType
	Parent           *ParentFieldPatch
	ChecklistInherit *bool
}

// Node is a task row plus nested children for API tree responses.
// Re-aliased by the public store facade as store.TaskNode.
type Node struct {
	domain.Task
	Children []Node `json:"children,omitempty" gorm:"-"`
}

// MaxTreeDepth bounds nesting depth for buildForest / GetTree
// responses. It MUST stay aligned with the maxTaskParseDepth value in
// web/src/api/parseTaskApi.ts; see internal/tasks/tree.go for the
// guard that returns ErrInvalidInput when exceeded.
const MaxTreeDepth = 64

const logCmd = "taskapi"
