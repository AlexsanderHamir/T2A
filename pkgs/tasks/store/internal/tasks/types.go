package tasks

import (
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// CreateInput is the task creation payload. Re-aliased by the public
// store facade as store.CreateTaskInput so handler code stays
// unchanged.
type CreateInput struct {
	ID                    string
	DraftID               string
	Title                 string
	InitialPrompt         string
	Status                domain.Status
	Priority              domain.Priority
	ProjectID             *string
	ProjectContextItemIDs []string
	Runner                string
	CursorModel           string
	// PickupNotBefore is optional; when set, the agent queue excludes this task
	// until the instant has passed (UTC).
	PickupNotBefore *time.Time
	Tags            []string
	Milestone       *string
	Gate            *domain.TaskGate
	DependsOn       []domain.DependencyEdge
}

// PickupNotBeforePatch updates pickup_not_before when non-nil. Clear true means
// set the column to NULL (the task is no longer scheduled). Re-aliased by the
// public store facade as store.PickupNotBeforePatch. See docs/data-model.md.
type PickupNotBeforePatch struct {
	Clear bool
	At    time.Time
}

// UpdateInput is the task patch payload. Each pointer field is
// optional; a nil pointer means "do not change". Re-aliased by the
// public store facade as store.UpdateTaskInput.
type UpdateInput struct {
	Title                 *string
	InitialPrompt         *string
	Status                *domain.Status
	Priority              *domain.Priority
	Project               *ProjectFieldPatch
	ProjectContextItemIDs *[]string
	PickupNotBefore       *PickupNotBeforePatch
	CursorModel           *string
	Tags                  *[]string
	Milestone             *string
	Gate                  **domain.TaskGate
	DependsOn             *[]domain.DependencyEdge
}

// ListFilter optionally restricts flat task listing.
type ListFilter struct {
	Tag       *string
	Milestone *string
}

// ProjectFieldPatch updates project_id when non-nil. Clear true means
// set project_id to null. Re-aliased by the public store facade as
// store.ProjectFieldPatch.
type ProjectFieldPatch struct {
	Clear bool
	ID    string
}

const logCmd = "taskapi"
