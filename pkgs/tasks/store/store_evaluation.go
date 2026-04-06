package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/google/uuid"
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

	out := &DraftTaskEvaluation{
		EvaluationID:        uuid.NewString(),
		CreatedAt:           time.Now().UTC(),
		OverallScore:        overall,
		OverallSummary:      overallSummary(overall),
		Sections:            sections,
		CohesionScore:       cohesionScore,
		CohesionSummary:     cohesionSummary(cohesionScore),
		CohesionSuggestions: randomSuggestions(rng, cohesionSuggestionsPool(), cohesionScore),
	}

	inputJSON, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal evaluation input: %w", err)
	}
	resultJSON, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal evaluation result: %w", err)
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
		return nil, fmt.Errorf("create draft evaluation: %w", err)
	}
	return out, nil
}

func (s *Store) ListDraftEvaluations(ctx context.Context, draftID string, limit int) ([]domain.TaskDraftEvaluation, error) {
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

func scoreTitle(title string) int {
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
	switch p {
	case domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical:
		return 92
	default:
		return 35
	}
}

func scoreStructure(parentID *string, inherit *bool, checklist []EvaluateDraftChecklistItemInput) int {
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
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func randomSuggestions(rng *rand.Rand, pool []string, score int) []string {
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
	if score >= 85 {
		return "Title is clear and specific."
	}
	if score >= 60 {
		return "Title is usable but could be tighter."
	}
	return "Title is too vague for reliable execution."
}

func promptSummary(score int) string {
	if score >= 85 {
		return "Prompt gives strong implementation context."
	}
	if score >= 60 {
		return "Prompt has useful detail, but misses precision."
	}
	return "Prompt lacks enough detail to guide implementation."
}

func prioritySummary(score int) string {
	if score >= 85 {
		return "Priority communicates urgency clearly."
	}
	return "Priority signal is missing or unclear."
}

func structureSummary(score int) string {
	if score >= 85 {
		return "Task structure supports predictable execution."
	}
	if score >= 60 {
		return "Structure is acceptable but can be improved."
	}
	return "Structure has scope and dependency ambiguity."
}

func cohesionSummary(score int) string {
	if score >= 85 {
		return "Sections align well and reinforce each other."
	}
	if score >= 60 {
		return "Most sections align, but intent can be sharpened."
	}
	return "Sections conflict or leave key gaps."
}

func overallSummary(score int) string {
	if score >= 85 {
		return "Strong draft, likely ready for creation."
	}
	if score >= 60 {
		return "Promising draft with a few improvement opportunities."
	}
	return "Draft needs revisions before creation."
}

func titleSuggestionsPool() []string {
	return []string{
		"Use a verb + object format in the title.",
		"Name the affected module or screen directly.",
		"Remove filler words and keep one clear outcome.",
		"Add a measurable success target in the title.",
	}
}

func promptSuggestionsPool() []string {
	return []string{
		"Add explicit acceptance criteria.",
		"Include edge cases that must be handled.",
		"Specify expected API or UI behavior.",
		"Clarify what is out of scope for this task.",
		"Describe the user-visible impact after completion.",
	}
}

func prioritySuggestionsPool() []string {
	return []string{
		"Set priority based on user impact and urgency.",
		"State why this priority level is justified.",
		"Align priority with current sprint goals.",
	}
}

func structureSuggestionsPool() []string {
	return []string{
		"Break implementation into a short checklist.",
		"Add dependency notes if this task relies on another item.",
		"Separate discovery work from execution tasks.",
		"Use checklist inheritance only when parent context is stable.",
	}
}

func cohesionSuggestionsPool() []string {
	return []string{
		"Ensure title, prompt, and priority describe the same outcome.",
		"Add one sentence linking technical work to user impact.",
		"Trim conflicting details that widen scope unexpectedly.",
		"Confirm checklist items map to acceptance criteria.",
	}
}
