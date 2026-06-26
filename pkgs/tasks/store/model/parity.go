package model

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ParityPair binds a domain struct prototype to its model counterpart for
// schema- and field-parity guards. Later phases append entries here.
type ParityPair struct {
	Name   string
	Domain any
	Model  any
	Table  string
	// DomainMigrateExtra lists additional domain structs AutoMigrate must run
	// before the primary domain type (e.g. parent tables for association FKs).
	// Schema comparison ignores FK constraints; extras exist so domain migrate
	// succeeds when associations reference other tables.
	DomainMigrateExtra []any
}

// ParityPairs is the single registry both parity tests iterate.
var ParityPairs = []ParityPair{
	{
		Name:   "AppSettings",
		Domain: &domain.AppSettings{},
		Model:  &AppSettings{},
		Table:  "app_settings",
	},
	{
		Name:   "TaskEvent",
		Domain: &domain.TaskEvent{},
		Model:  &TaskEvent{},
		Table:  "task_events",
		DomainMigrateExtra: []any{
			&domain.Task{},
		},
	},
	{
		Name:   "Task",
		Domain: &domain.Task{},
		Model:  &Task{},
		Table:  "tasks",
		DomainMigrateExtra: []any{
			&domain.Project{},
		},
	},
	{
		Name:   "TaskDependency",
		Domain: &domain.TaskDependency{},
		Model:  &TaskDependency{},
		Table:  "task_dependencies",
		DomainMigrateExtra: []any{
			&domain.Task{},
		},
	},
	{
		Name:   "Project",
		Domain: &domain.Project{},
		Model:  &Project{},
		Table:  "projects",
	},
	{
		Name:   "ProjectContextItem",
		Domain: &domain.ProjectContextItem{},
		Model:  &ProjectContextItem{},
		Table:  "project_context_items",
		DomainMigrateExtra: []any{
			&domain.Project{},
			&domain.Task{},
			&domain.TaskCycle{},
		},
	},
	{
		Name:   "ProjectContextEdge",
		Domain: &domain.ProjectContextEdge{},
		Model:  &ProjectContextEdge{},
		Table:  "project_context_edges",
		DomainMigrateExtra: []any{
			&domain.Project{},
			&domain.ProjectContextItem{},
		},
	},
	{
		Name:   "TaskContextSnapshot",
		Domain: &domain.TaskContextSnapshot{},
		Model:  &TaskContextSnapshot{},
		Table:  "task_context_snapshots",
		DomainMigrateExtra: []any{
			&domain.Task{},
			&domain.TaskCycle{},
			&domain.Project{},
		},
	},
	{
		Name:   "TaskChecklistItem",
		Domain: &domain.TaskChecklistItem{},
		Model:  &TaskChecklistItem{},
		Table:  "task_checklist_items",
		DomainMigrateExtra: []any{
			&domain.Task{},
		},
	},
	{
		Name:   "TaskChecklistCompletion",
		Domain: &domain.TaskChecklistCompletion{},
		Model:  &TaskChecklistCompletion{},
		Table:  "task_checklist_completions",
		DomainMigrateExtra: []any{
			&domain.Task{},
			&domain.TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskChecklistItemCommand",
		Domain: &domain.TaskChecklistItemCommand{},
		Model:  &TaskChecklistItemCommand{},
		Table:  "task_checklist_item_commands",
		DomainMigrateExtra: []any{
			&domain.TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskCycle",
		Domain: &domain.TaskCycle{},
		Model:  &TaskCycle{},
		Table:  "task_cycles",
		DomainMigrateExtra: []any{
			&domain.Task{},
		},
	},
	{
		Name:   "TaskCyclePhase",
		Domain: &domain.TaskCyclePhase{},
		Model:  &TaskCyclePhase{},
		Table:  "task_cycle_phases",
		DomainMigrateExtra: []any{
			&domain.Task{},
			&domain.TaskCycle{},
		},
	},
	{
		Name:   "TaskCycleStreamEvent",
		Domain: &domain.TaskCycleStreamEvent{},
		Model:  &TaskCycleStreamEvent{},
		Table:  "task_cycle_stream_events",
		DomainMigrateExtra: []any{
			&domain.Task{},
			&domain.TaskCycle{},
		},
	},
}
