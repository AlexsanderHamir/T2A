package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type renderedProjectContext struct {
	Text          string
	TokenEstimate int
	SnapshotJSON  json.RawMessage
}

func (w *Worker) selectedProjectContext(ctx context.Context, task *domain.Task, cycle *domain.TaskCycle) (renderedProjectContext, error) {
	if task.ProjectID == nil || strings.TrimSpace(*task.ProjectID) == "" || len(task.ProjectContextItemIDs) == 0 {
		return renderedProjectContext{}, nil
	}
	project, err := w.store.GetProject(ctx, *task.ProjectID)
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("get project: %w", err)
	}
	items, err := w.store.ListProjectContextByIDs(ctx, project.ID, task.ProjectContextItemIDs)
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("list selected context: %w", err)
	}
	rendered := renderProjectContext(project, items)
	raw, err := json.Marshal(map[string]any{
		"project_id": project.ID,
		"project": map[string]string{
			"id":              project.ID,
			"name":            project.Name,
			"context_summary": project.ContextSummary,
		},
		"selected_item_ids": task.ProjectContextItemIDs,
		"items":             items,
	})
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("marshal context snapshot: %w", err)
	}
	_, err = w.store.CreateTaskContextSnapshot(ctx, store.CreateTaskContextSnapshotInput{
		TaskID:          task.ID,
		CycleID:         cycle.ID,
		ProjectID:       project.ID,
		ContextJSON:     raw,
		RenderedContext: rendered,
		TokenEstimate:   estimateTokens(rendered),
	})
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("create context snapshot: %w", err)
	}
	return renderedProjectContext{
		Text:          rendered,
		TokenEstimate: estimateTokens(rendered),
		SnapshotJSON:  raw,
	}, nil
}

func renderProjectContext(project domain.Project, items []domain.ProjectContextItem) string {
	var b strings.Builder
	b.WriteString("<project_context>\n")
	b.WriteString("Project: ")
	b.WriteString(project.Name)
	b.WriteString("\n")
	if strings.TrimSpace(project.ContextSummary) != "" {
		b.WriteString("Summary: ")
		b.WriteString(strings.TrimSpace(project.ContextSummary))
		b.WriteString("\n")
	}
	for _, item := range items {
		b.WriteString("\n[")
		b.WriteString(string(item.Kind))
		b.WriteString("] ")
		b.WriteString(item.Title)
		b.WriteString("\n")
		b.WriteString(item.Body)
		b.WriteString("\n")
	}
	b.WriteString("</project_context>")
	return b.String()
}

func estimateTokens(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return (len([]rune(s)) + 3) / 4
}

func promptWithProjectContext(prompt string, projectContext string) string {
	if strings.TrimSpace(projectContext) == "" {
		return prompt
	}
	return projectContext + "\n\n<task_prompt>\n" + prompt + "\n</task_prompt>"
}
