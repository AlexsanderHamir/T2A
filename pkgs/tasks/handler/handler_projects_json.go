package handler

import "github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"

type projectCreateJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	ContextSummary string `json:"context_summary"`
}

type projectPatchJSON struct {
	Name           *string               `json:"name"`
	Description    *string               `json:"description"`
	Status         *domain.ProjectStatus `json:"status"`
	ContextSummary *string               `json:"context_summary"`
}

func (p projectPatchJSON) isEmpty() bool {
	return p.Name == nil && p.Description == nil && p.Status == nil && p.ContextSummary == nil
}

type projectsListResponse struct {
	Projects []domain.Project `json:"projects"`
	Limit    int              `json:"limit"`
}

type projectContextCreateJSON struct {
	ID            string                    `json:"id"`
	Kind          domain.ProjectContextKind `json:"kind"`
	Title         string                    `json:"title"`
	Body          string                    `json:"body"`
	SourceTaskID  *string                   `json:"source_task_id"`
	SourceCycleID *string                   `json:"source_cycle_id"`
	Pinned        bool                      `json:"pinned"`
}

type projectContextPatchJSON struct {
	Kind   *domain.ProjectContextKind `json:"kind"`
	Title  *string                    `json:"title"`
	Body   *string                    `json:"body"`
	Pinned *bool                      `json:"pinned"`
}

func (p projectContextPatchJSON) isEmpty() bool {
	return p.Kind == nil && p.Title == nil && p.Body == nil && p.Pinned == nil
}

type projectContextListResponse struct {
	Items []domain.ProjectContextItem `json:"items"`
	Limit int                         `json:"limit"`
}
