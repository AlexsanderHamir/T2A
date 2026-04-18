package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/eval"
)

// EvaluateDraftChecklistItemInput is the public re-export of the
// per-item checklist input used by the draft evaluation rubric.
type EvaluateDraftChecklistItemInput = eval.ChecklistItemInput

// EvaluateDraftTaskInput is the public re-export of the rubric's
// draft snapshot input.
type EvaluateDraftTaskInput = eval.DraftTaskInput

// DraftEvaluationSection is the public re-export of one rubric facet
// in the evaluation result.
type DraftEvaluationSection = eval.Section

// DraftTaskEvaluation is the public re-export of the rubric output
// persisted in task_draft_evaluations.result_json.
type DraftTaskEvaluation = eval.Result

// EvaluateDraftTask scores task-creation input and persists each
// evaluation. See internal/eval for the rubric.
func (s *Store) EvaluateDraftTask(ctx context.Context, in EvaluateDraftTaskInput, by domain.Actor) (*DraftTaskEvaluation, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.EvaluateDraftTask")
	return eval.EvaluateDraftTask(ctx, s.db, in, by)
}

// ListDraftEvaluations returns the most-recent evaluations for
// draftID, newest first.
func (s *Store) ListDraftEvaluations(ctx context.Context, draftID string, limit int) ([]domain.TaskDraftEvaluation, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDraftEvaluations")
	return eval.ListDraftEvaluations(ctx, s.db, draftID, limit)
}
