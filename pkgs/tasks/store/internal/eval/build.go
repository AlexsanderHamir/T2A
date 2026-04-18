package eval

import (
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// buildResult assembles the rubric Result from the deterministic
// per-section scoring functions and the wall-clock-seeded suggestion
// sampler. Pure (no I/O) so persistRow can store the byte stream and
// return it to the caller in one step.
func buildResult(in DraftTaskInput, rng *rand.Rand) *Result {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.eval.buildResult")
	titleScore := scoreTitle(strings.TrimSpace(in.Title))
	promptScore := scorePrompt(strings.TrimSpace(in.InitialPrompt))
	priorityScore := scorePriority(in.Priority)
	structureScore := scoreStructure(in.ParentID, in.ChecklistInherit, in.ChecklistItems)
	cohesionScore := scoreCohesion(titleScore, promptScore, priorityScore, structureScore, in)
	overall := clampScore(int(math.Round((float64(titleScore) + float64(promptScore) + float64(priorityScore) + float64(structureScore) + float64(cohesionScore)) / 5.0)))

	sections := []Section{
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

	return &Result{
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
