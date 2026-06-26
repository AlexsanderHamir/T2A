package model

import (
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ParityPair binds a domain struct prototype to its model counterpart for
// schema- and field-parity guards.
type ParityPair struct {
	Name   string
	Domain any
	Model  any
	Table  string
	// ModelMigrateExtra lists additional model structs AutoMigrate must run
	// before the primary model type (e.g. parent tables for association FKs).
	ModelMigrateExtra []any
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
		ModelMigrateExtra: []any{
			&Task{},
		},
	},
	{
		Name:   "Task",
		Domain: &domain.Task{},
		Model:  &Task{},
		Table:  "tasks",
		ModelMigrateExtra: []any{
			&Project{},
		},
	},
	{
		Name:   "TaskDependency",
		Domain: &domain.TaskDependency{},
		Model:  &TaskDependency{},
		Table:  "task_dependencies",
		ModelMigrateExtra: []any{
			&Task{},
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
		ModelMigrateExtra: []any{
			&Project{},
			&Task{},
			&TaskCycle{},
		},
	},
	{
		Name:   "ProjectContextEdge",
		Domain: &domain.ProjectContextEdge{},
		Model:  &ProjectContextEdge{},
		Table:  "project_context_edges",
		ModelMigrateExtra: []any{
			&Project{},
			&ProjectContextItem{},
		},
	},
	{
		Name:   "TaskContextSnapshot",
		Domain: &domain.TaskContextSnapshot{},
		Model:  &TaskContextSnapshot{},
		Table:  "task_context_snapshots",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
			&Project{},
		},
	},
	{
		Name:   "TaskChecklistItem",
		Domain: &domain.TaskChecklistItem{},
		Model:  &TaskChecklistItem{},
		Table:  "task_checklist_items",
		ModelMigrateExtra: []any{
			&Task{},
		},
	},
	{
		Name:   "TaskChecklistCompletion",
		Domain: &domain.TaskChecklistCompletion{},
		Model:  &TaskChecklistCompletion{},
		Table:  "task_checklist_completions",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskChecklistItemCommand",
		Domain: &domain.TaskChecklistItemCommand{},
		Model:  &TaskChecklistItemCommand{},
		Table:  "task_checklist_item_commands",
		ModelMigrateExtra: []any{
			&TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskCycle",
		Domain: &domain.TaskCycle{},
		Model:  &TaskCycle{},
		Table:  "task_cycles",
		ModelMigrateExtra: []any{
			&Task{},
		},
	},
	{
		Name:   "TaskCyclePhase",
		Domain: &domain.TaskCyclePhase{},
		Model:  &TaskCyclePhase{},
		Table:  "task_cycle_phases",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
		},
	},
	{
		Name:   "TaskCycleStreamEvent",
		Domain: &domain.TaskCycleStreamEvent{},
		Model:  &TaskCycleStreamEvent{},
		Table:  "task_cycle_stream_events",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
		},
	},
	{
		Name:   "TaskCycleCriteriaReport",
		Domain: &domain.TaskCycleCriteriaReport{},
		Model:  &TaskCycleCriteriaReport{},
		Table:  "task_cycle_criteria_reports",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
			&TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskCycleVerifyReport",
		Domain: &domain.TaskCycleVerifyReport{},
		Model:  &TaskCycleVerifyReport{},
		Table:  "task_cycle_verify_reports",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
			&TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskCycleCommandRun",
		Domain: &domain.TaskCycleCommandRun{},
		Model:  &TaskCycleCommandRun{},
		Table:  "task_cycle_command_runs",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
			&TaskChecklistItem{},
		},
	},
	{
		Name:   "TaskCycleCommit",
		Domain: &domain.TaskCycleCommit{},
		Model:  &TaskCycleCommit{},
		Table:  "task_cycle_commits",
		ModelMigrateExtra: []any{
			&Task{},
			&TaskCycle{},
		},
	},
	{
		Name:   "TaskDraft",
		Domain: &domain.TaskDraft{},
		Model:  &TaskDraft{},
		Table:  "task_drafts",
	},
	{
		Name:   "TaskTemplate",
		Domain: &domain.TaskTemplate{},
		Model:  &TaskTemplate{},
		Table:  "task_templates",
	},
	{
		Name:   "GitRepository",
		Domain: &domain.GitRepository{},
		Model:  &GitRepository{},
		Table:  "git_repositories",
	},
	{
		Name:   "GitWorktree",
		Domain: &domain.GitWorktree{},
		Model:  &GitWorktree{},
		Table:  "git_worktrees",
		ModelMigrateExtra: []any{
			&GitRepository{},
		},
	},
	{
		Name:   "GitBranch",
		Domain: &domain.GitBranch{},
		Model:  &GitBranch{},
		Table:  "git_branches",
		ModelMigrateExtra: []any{
			&GitRepository{},
		},
	},
	{
		Name:   "WorktreeBranch",
		Domain: &domain.WorktreeBranch{},
		Model:  &WorktreeBranch{},
		Table:  "worktree_branches",
		ModelMigrateExtra: []any{
			&GitWorktree{},
			&GitBranch{},
		},
	},
}
