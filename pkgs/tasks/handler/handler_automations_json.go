package handler

import (
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type automationCreateJSON struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type automationPatchJSON struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (p automationPatchJSON) isEmpty() bool {
	return p.Title == nil && p.Description == nil
}

type automationsListResponse struct {
	Automations []domain.Automation `json:"automations"`
	Limit       int                 `json:"limit"`
}

type automationSelectionJSON struct {
	AutomationID string                 `json:"automation_id"`
	State        domain.AutomationState `json:"state"`
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseAutomationSelectionsWire(raw []automationSelectionJSON) ([]domain.AutomationSelection, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	in := make([]domain.AutomationSelection, len(raw))
	for i, row := range raw {
		in[i] = domain.AutomationSelection{
			AutomationID: row.AutomationID,
			State:        row.State,
		}
	}
	return domain.ValidateAutomationSelections(in)
}
