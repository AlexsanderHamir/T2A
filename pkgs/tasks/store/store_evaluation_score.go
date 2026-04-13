package store

import (
	"log/slog"
	"math"
	"math/rand"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func scoreTitle(title string) int {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.scoreTitle")
	switch n := len([]rune(title)); {
	case n == 0:
		return 15
	case n < 10:
		return 45
	case n <= 72:
		return 88
	case n <= 120:
		return 74
	default:
		return 58
	}
}

func scorePrompt(prompt string) int {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.scorePrompt")
	switch n := len([]rune(prompt)); {
	case n == 0:
		return 20
	case n < 40:
		return 48
	case n <= 400:
		return 90
	case n <= 1200:
		return 76
	default:
		return 62
	}
}

func scorePriority(p domain.Priority) int {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.scorePriority")
	switch p {
	case domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical:
		return 92
	default:
		return 35
	}
}

func scoreStructure(parentID *string, inherit *bool, checklist []EvaluateDraftChecklistItemInput) int {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.scoreStructure")
	score := 72
	if parentID != nil && strings.TrimSpace(*parentID) != "" {
		score += 10
	}
	if inherit != nil && *inherit && (parentID == nil || strings.TrimSpace(*parentID) == "") {
		score -= 25
	}
	nonEmptyChecklist := 0
	for _, item := range checklist {
		if strings.TrimSpace(item.Text) != "" {
			nonEmptyChecklist++
		}
	}
	switch {
	case nonEmptyChecklist == 0:
		score -= 10
	case nonEmptyChecklist <= 3:
		score += 8
	default:
		score += 12
	}
	return clampScore(score)
}

func scoreCohesion(title, prompt, priority, structure int, in EvaluateDraftTaskInput) int {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.scoreCohesion")
	score := int(math.Round((float64(title) + float64(prompt) + float64(priority) + float64(structure)) / 4.0))
	if strings.TrimSpace(in.Title) != "" && strings.TrimSpace(in.InitialPrompt) == "" {
		score -= 18
	}
	if strings.TrimSpace(in.InitialPrompt) != "" && in.Priority == "" {
		score -= 14
	}
	return clampScore(score)
}

func clampScore(v int) int {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.clampScore")
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func randomSuggestions(rng *rand.Rand, pool []string, score int) []string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.randomSuggestions")
	if len(pool) == 0 {
		return []string{"Refine this section with clearer intent."}
	}
	count := 2
	switch {
	case score >= 85:
		count = 1
	case score >= 60:
		count = 2
	default:
		count = 3
	}
	if count > len(pool) {
		count = len(pool)
	}
	perm := rng.Perm(len(pool))
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, pool[perm[i]])
	}
	return out
}

func titleSummary(score int) string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.titleSummary")
	if score >= 85 {
		return "Title is clear and specific."
	}
	if score >= 60 {
		return "Title is usable but could be tighter."
	}
	return "Title is too vague for reliable execution."
}

func promptSummary(score int) string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.promptSummary")
	if score >= 85 {
		return "Prompt gives strong implementation context."
	}
	if score >= 60 {
		return "Prompt has useful detail, but misses precision."
	}
	return "Prompt lacks enough detail to guide implementation."
}

func prioritySummary(score int) string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.prioritySummary")
	if score >= 85 {
		return "Priority communicates urgency clearly."
	}
	return "Priority signal is missing or unclear."
}

func structureSummary(score int) string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.structureSummary")
	if score >= 85 {
		return "Task structure supports predictable execution."
	}
	if score >= 60 {
		return "Structure is acceptable but can be improved."
	}
	return "Structure has scope and dependency ambiguity."
}

func cohesionSummary(score int) string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.cohesionSummary")
	if score >= 85 {
		return "Sections align well and reinforce each other."
	}
	if score >= 60 {
		return "Most sections align, but intent can be sharpened."
	}
	return "Sections conflict or leave key gaps."
}

func overallSummary(score int) string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.overallSummary")
	if score >= 85 {
		return "Strong draft, likely ready for creation."
	}
	if score >= 60 {
		return "Promising draft with a few improvement opportunities."
	}
	return "Draft needs revisions before creation."
}

func titleSuggestionsPool() []string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.titleSuggestionsPool")
	return []string{
		"Use a verb + object format in the title.",
		"Name the affected module or screen directly.",
		"Remove filler words and keep one clear outcome.",
		"Add a measurable success target in the title.",
	}
}

func promptSuggestionsPool() []string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.promptSuggestionsPool")
	return []string{
		"Add explicit acceptance criteria.",
		"Include edge cases that must be handled.",
		"Specify expected API or UI behavior.",
		"Clarify what is out of scope for this task.",
		"Describe the user-visible impact after completion.",
	}
}

func prioritySuggestionsPool() []string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.prioritySuggestionsPool")
	return []string{
		"Set priority based on user impact and urgency.",
		"State why this priority level is justified.",
		"Align priority with current sprint goals.",
	}
}

func structureSuggestionsPool() []string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.structureSuggestionsPool")
	return []string{
		"Break implementation into a short checklist.",
		"Add dependency notes if this task relies on another item.",
		"Separate discovery work from execution tasks.",
		"Use checklist inheritance only when parent context is stable.",
	}
}

func cohesionSuggestionsPool() []string {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.cohesionSuggestionsPool")
	return []string{
		"Ensure title, prompt, and priority describe the same outcome.",
		"Add one sentence linking technical work to user impact.",
		"Trim conflicting details that widen scope unexpectedly.",
		"Confirm checklist items map to acceptance criteria.",
	}
}
