package store

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

type EvaluateDraftChecklistItemInput struct {
	Text string `json:"text"`
}

type EvaluateDraftTaskInput struct {
	DraftID          string
	Title            string
	InitialPrompt    string
	Status           domain.Status
	Priority         domain.Priority
	TaskType         domain.TaskType
	ParentID         *string
	ChecklistInherit *bool
	ChecklistItems   []EvaluateDraftChecklistItemInput
}

type DraftEvaluationSection struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Score       int      `json:"score"`
	Summary     string   `json:"summary"`
	Suggestions []string `json:"suggestions"`
}

type DraftTaskEvaluation struct {
	EvaluationID        string                   `json:"evaluation_id"`
	CreatedAt           time.Time                `json:"created_at"`
	OverallScore        int                      `json:"overall_score"`
	OverallSummary      string                   `json:"overall_summary"`
	Sections            []DraftEvaluationSection `json:"sections"`
	CohesionScore       int                      `json:"cohesion_score"`
	CohesionSummary     string                   `json:"cohesion_summary"`
	CohesionSuggestions []string                 `json:"cohesion_suggestions"`
}

// EvaluateDraftTask scores task-creation input and persists each evaluation.
func (s *Store) EvaluateDraftTask(ctx context.Context, in EvaluateDraftTaskInput, by domain.Actor) (*DraftTaskEvaluation, error) {
	defer deferStoreLatency(storeOpEvaluateDraft)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.EvaluateDraftTask")
	if err := validateActor(by); err != nil {
		return nil, err
	}
	tt := in.TaskType
	if tt == "" {
		tt = domain.TaskTypeGeneral
	}
	if !validTaskType(tt) {
		return nil, fmt.Errorf("%w: invalid task_type", domain.ErrInvalidInput)
	}
	in.TaskType = tt
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	out := buildDraftTaskEvaluationModel(in, rng)
	if err := s.persistDraftEvaluationRow(ctx, in, by, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListDraftEvaluations(ctx context.Context, draftID string, limit int) ([]domain.TaskDraftEvaluation, error) {
	defer deferStoreLatency(storeOpListDraftEvaluations)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDraftEvaluations")
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
	err := s.db.WithContext(ctx).
		Where("draft_id = ?", draftID).
		Order("created_at DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list draft evaluations: %w", err)
	}
	return out, nil
}

func attachDraftEvaluationsTx(tx *gorm.DB, draftID string, taskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.attachDraftEvaluationsTx")
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
