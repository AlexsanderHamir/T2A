package domain

import "time"

// DefaultProjectID is the built-in project available on every installation.
const DefaultProjectID = "00000000-0000-4000-8000-000000000001"

// DefaultProject returns the non-deletable project every workspace starts with.
func DefaultProject(now time.Time) Project {
	return Project{
		ID:             DefaultProjectID,
		Name:           "Default project",
		Description:    "Built-in project for general task context.",
		Status:         ProjectStatusActive,
		ContextSummary: "Shared context selected for tasks that do not need a custom project.",
		CreatedAt:      now.UTC(),
		UpdatedAt:      now.UTC(),
	}
}
