package eval

import (
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ChecklistItemInput is one acceptance-criteria line in the draft
// snapshot fed to the rubric. Re-aliased by the public store facade
// as store.EvaluateDraftChecklistItemInput.
type ChecklistItemInput struct {
	Text string `json:"text"`
}

// DraftTaskInput is the read-only draft snapshot fed to the rubric.
// Re-aliased by the public store facade as
// store.EvaluateDraftTaskInput so handler code stays unchanged.
type DraftTaskInput struct {
	DraftID          string
	Title            string
	InitialPrompt    string
	Status           domain.Status
	Priority         domain.Priority
	TaskType         domain.TaskType
	ParentID         *string
	ChecklistInherit *bool
	ChecklistItems   []ChecklistItemInput
}

// Section is one rubric facet of the evaluation result. The Key is
// stable across runs so the UI can address sections by name.
type Section struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Score       int      `json:"score"`
	Summary     string   `json:"summary"`
	Suggestions []string `json:"suggestions"`
}

// Result is the rubric output persisted in
// task_draft_evaluations.result_json. Re-aliased by the public store
// facade as store.DraftTaskEvaluation.
type Result struct {
	EvaluationID        string    `json:"evaluation_id"`
	CreatedAt           time.Time `json:"created_at"`
	OverallScore        int       `json:"overall_score"`
	OverallSummary      string    `json:"overall_summary"`
	Sections            []Section `json:"sections"`
	CohesionScore       int       `json:"cohesion_score"`
	CohesionSummary     string    `json:"cohesion_summary"`
	CohesionSuggestions []string  `json:"cohesion_suggestions"`
}
