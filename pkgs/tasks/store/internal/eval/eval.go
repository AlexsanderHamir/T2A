package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

// EvaluateDraftTask scores a task-creation draft and persists one
// task_draft_evaluations row per call. Returns the rubric result with
// a freshly minted EvaluationID so callers can correlate the row with
// follow-up writes (e.g., AttachDraftEvaluationsInTx after Create).
func EvaluateDraftTask(ctx context.Context, db *gorm.DB, in DraftTaskInput, by domain.Actor) (*Result, error) {
	defer kernel.DeferLatency(kernel.OpEvaluateDraft)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.eval.EvaluateDraftTask")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, err
	}
	tt := in.TaskType
	if tt == "" {
		tt = domain.TaskTypeGeneral
	}
	if !kernel.ValidTaskType(tt) {
		return nil, fmt.Errorf("%w: invalid task_type", domain.ErrInvalidInput)
	}
	in.TaskType = tt
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	out := buildResult(in, rng)
	if err := persistRow(ctx, db, in, by, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListDraftEvaluations returns the most-recent task_draft_evaluations
// rows for draftID, newest first; limit is clamped to [1, 200].
func ListDraftEvaluations(ctx context.Context, db *gorm.DB, draftID string, limit int) ([]domain.TaskDraftEvaluation, error) {
	defer kernel.DeferLatency(kernel.OpListDraftEvaluations)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.eval.ListDraftEvaluations")
	draftID = strings.TrimSpace(draftID)
	if draftID == "" {
		return nil, fmt.Errorf("%w: draft_id", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var out []domain.TaskDraftEvaluation
	err := db.WithContext(ctx).
		Where("draft_id = ?", draftID).
		Order("created_at DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list draft evaluations: %w", err)
	}
	return out, nil
}

// AttachDraftEvaluationsInTx links every task_draft_evaluations row
// recorded against draftID (and not yet linked to a task) to taskID.
// No-op when either id is empty so the tasks-CRUD subpackage can call
// this unconditionally inside its Create transaction.
func AttachDraftEvaluationsInTx(tx *gorm.DB, draftID string, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.eval.AttachDraftEvaluationsInTx")
	draftID = strings.TrimSpace(draftID)
	taskID = strings.TrimSpace(taskID)
	if draftID == "" || taskID == "" {
		return nil
	}
	if err := tx.Model(&domain.TaskDraftEvaluation{}).
		Where("draft_id = ? AND task_id IS NULL", draftID).
		Update("task_id", taskID).Error; err != nil {
		return fmt.Errorf("attach draft evaluations: %w", err)
	}
	return nil
}

func persistRow(ctx context.Context, db *gorm.DB, in DraftTaskInput, by domain.Actor, out *Result) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.eval.persistRow")
	inputJSON, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal evaluation input: %w", err)
	}
	resultJSON, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal evaluation result: %w", err)
	}
	row := domain.TaskDraftEvaluation{
		ID:         out.EvaluationID,
		By:         by,
		InputJSON:  inputJSON,
		ResultJSON: resultJSON,
		CreatedAt:  out.CreatedAt,
	}
	if d := strings.TrimSpace(in.DraftID); d != "" {
		row.DraftID = &d
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create draft evaluation: %w", err)
	}
	return nil
}
