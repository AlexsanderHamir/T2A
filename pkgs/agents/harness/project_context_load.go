package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type renderedProjectContext struct {
	Text          string
	TokenEstimate int
	SnapshotJSON  json.RawMessage
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Harness) selectedProjectContext(ctx context.Context, task *domain.Task, cycle *domain.TaskCycle) (renderedProjectContext, error) {
	if task.ProjectID == nil || strings.TrimSpace(*task.ProjectID) == "" || len(task.ProjectContextItemIDs) == 0 {
		return renderedProjectContext{}, nil
	}
	project, err := h.store.GetProject(ctx, *task.ProjectID)
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("get project: %w", err)
	}
	items, err := h.store.ListProjectContextByIDs(ctx, project.ID, task.ProjectContextItemIDs)
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("list selected context: %w", err)
	}
	edges, err := h.store.ListProjectContextEdges(ctx, project.ID, task.ProjectContextItemIDs)
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("list selected context edges: %w", err)
	}
	rendered := prompt.BuildProjectContextSection(prompt.ProjectContextInput{
		Project: project,
		Items:   items,
		Edges:   edges,
	})
	raw, err := json.Marshal(map[string]any{
		"project_id": project.ID,
		"project": map[string]string{
			"id":              project.ID,
			"name":            project.Name,
			"context_summary": project.ContextSummary,
		},
		"selected_item_ids": task.ProjectContextItemIDs,
		"items":             items,
		"edges":             edges,
	})
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("marshal context snapshot: %w", err)
	}
	if existing, err := h.store.GetTaskContextSnapshotForCycle(ctx, cycle.ID); err == nil {
		return renderedProjectContext{
			Text:          existing.RenderedContext,
			TokenEstimate: existing.TokenEstimate,
			SnapshotJSON:  json.RawMessage(existing.ContextJSON),
		}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return renderedProjectContext{}, fmt.Errorf("get context snapshot: %w", err)
	}
	_, err = h.store.CreateTaskContextSnapshot(ctx, store.CreateTaskContextSnapshotInput{
		TaskID:          task.ID,
		CycleID:         cycle.ID,
		ProjectID:       project.ID,
		ContextJSON:     raw,
		RenderedContext: rendered,
		TokenEstimate:   prompt.EstimateTokens(rendered),
	})
	if err != nil {
		return renderedProjectContext{}, fmt.Errorf("create context snapshot: %w", err)
	}
	return renderedProjectContext{
		Text:          rendered,
		TokenEstimate: prompt.EstimateTokens(rendered),
		SnapshotJSON:  raw,
	}, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func checklistItemsForPrompt(items []store.ChecklistVerifyItem) []prompt.ChecklistItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]prompt.ChecklistItem, len(items))
	for i, it := range items {
		out[i] = prompt.ChecklistItem{ID: it.ID, Text: it.Text}
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func verifiedCriterionIDs(previouslyPassed map[string]criterionVerdict) map[string]struct{} {
	if len(previouslyPassed) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(previouslyPassed))
	for id := range previouslyPassed {
		out[id] = struct{}{}
	}
	return out
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func continuationInputFromBundle(cycle *domain.TaskCycle, bundle *ContinuationBundle) prompt.ContinuationInput {
	if bundle == nil {
		return prompt.ContinuationInput{Cycle: cycle}
	}
	return prompt.ContinuationInput{
		LineageAttempt:  bundle.LineageAttempt,
		Cycle:           cycle,
		FailureClass:    string(bundle.FailureClass),
		FailureReason:   bundle.FailureReason,
		FailurePhase:    bundle.FailurePhase,
		ScopeFiles:      bundle.ScopeFiles,
		Commits:         bundle.Commits,
		ExecuteFeedback: bundle.ExecuteFeedback,
		RunnerFeedback:  bundle.RunnerFeedback,
		GitDiagnostics:  bundle.GitDiagnostics,
		Warnings:        bundle.Warnings,
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func cycleIDOrEmpty(cycle *domain.TaskCycle) string {
	if cycle == nil {
		return ""
	}
	return cycle.ID
}
