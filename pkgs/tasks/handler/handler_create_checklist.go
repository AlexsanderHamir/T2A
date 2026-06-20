package handler

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// parseCreateChecklistItems normalizes POST /tasks checklist_items: trims text,
// drops blanks, and requires at least one surviving criterion.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseCreateChecklistItems(items []store.CreateChecklistItemInput) ([]store.CreateChecklistItemInput, error) {
	var out []store.CreateChecklistItemInput
	for _, it := range items {
		t := strings.TrimSpace(it.Text)
		if t == "" {
			continue
		}
		cmds := make([]store.VerifyCommandInput, 0, len(it.VerifyCommands))
		for _, c := range it.VerifyCommands {
			cmds = append(cmds, store.VerifyCommandInput{
				Command:         c.Command,
				ExpectedOutcome: c.ExpectedOutcome,
			})
		}
		normalized, err := store.NormalizeVerifyCommands(cmds)
		if err != nil {
			return nil, err
		}
		out = append(out, store.CreateChecklistItemInput{
			Text:           t,
			VerifyCommands: normalized,
		})
	}
	if len(out) < 1 {
		return nil, fmt.Errorf("%w: at least one done criterion required", domain.ErrInvalidInput)
	}
	return out, nil
}
