package handler

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// parseCreateChecklistItems normalizes POST /tasks checklist_items: trims text,
// drops blanks, and requires at least one surviving criterion.
func parseCreateChecklistItems(items []store.EvaluateDraftChecklistItemInput) ([]string, error) {
	var out []string
	for _, it := range items {
		t := strings.TrimSpace(it.Text)
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) < 1 {
		return nil, fmt.Errorf("%w: at least one done criterion required", domain.ErrInvalidInput)
	}
	return out, nil
}
