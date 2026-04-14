package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/google/uuid"
)

func buildDraftTaskEvaluationModel(in EvaluateDraftTaskInput, rng *rand.Rand) *DraftTaskEvaluation {
	titleScore := scoreTitle(strings.TrimSpace(in.Title))
	promptScore := scorePrompt(strings.TrimSpace(in.InitialPrompt))
	priorityScore := scorePriority(in.Priority)
	structureScore := scoreStructure(in.ParentID, in.ChecklistInherit, in.ChecklistItems)
	cohesionScore := scoreCohesion(titleScore, promptScore, priorityScore, structureScore, in)
	overall := clampScore(int(math.Round((float64(titleScore) + float64(promptScore) + float64(priorityScore) + float64(structureScore) + float64(cohesionScore)) / 5.0)))

	sections := []DraftEvaluationSection{
		{
			Key:         "title",
			Label:       "Title quality",
			Score:       titleScore,
			Summary:     titleSummary(titleScore),
			Suggestions: randomSuggestions(rng, titleSuggestionsPool(), titleScore),
		},
		{
			Key:         "initial_prompt",
			Label:       "Prompt clarity",
			Score:       promptScore,
			Summary:     promptSummary(promptScore),
			Suggestions: randomSuggestions(rng, promptSuggestionsPool(), promptScore),
		},
		{
			Key:         "priority",
			Label:       "Priority signal",
			Score:       priorityScore,
			Summary:     prioritySummary(priorityScore),
			Suggestions: randomSuggestions(rng, prioritySuggestionsPool(), priorityScore),
		},
		{
			Key:         "structure",
			Label:       "Structure and scope",
			Score:       structureScore,
			Summary:     structureSummary(structureScore),
			Suggestions: randomSuggestions(rng, structureSuggestionsPool(), structureScore),
		},
	}

	return &DraftTaskEvaluation{
		EvaluationID:        uuid.NewString(),
		CreatedAt:           time.Now().UTC(),
		OverallScore:        overall,
		OverallSummary:      overallSummary(overall),
		Sections:            sections,
		CohesionScore:       cohesionScore,
		CohesionSummary:     cohesionSummary(cohesionScore),
		CohesionSuggestions: randomSuggestions(rng, cohesionSuggestionsPool(), cohesionScore),
	}
}

func (s *Store) persistDraftEvaluationRow(ctx context.Context, in EvaluateDraftTaskInput, by domain.Actor, out *DraftTaskEvaluation) error {
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
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create draft evaluation: %w", err)
	}
	return nil
}
