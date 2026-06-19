package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	// MaxAutomationSelectionsPerTask caps prompt size from behavior toggles.
	MaxAutomationSelectionsPerTask = 20
	maxAutomationTitleLen          = 200
	maxAutomationDescriptionLen    = 2000
)

// AutomationState is the per-task toggle for a library automation.
type AutomationState string

const (
	AutomationStateYes AutomationState = "yes"
	AutomationStateNo  AutomationState = "no"
)

// Automation is a global reusable behavioral instruction in the library.
type Automation struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Title       string     `json:"title" gorm:"not null;index"`
	Description string     `json:"description" gorm:"type:text;not null"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null;index"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"not null;index"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty" gorm:"index"`
}

// TableName implements gorm tabler.
func (Automation) TableName() string { return "automations" }

// AutomationSelection binds one library automation to a task with yes/no.
type AutomationSelection struct {
	AutomationID string          `json:"automation_id"`
	State        AutomationState `json:"state"`
}

// ResolvedAutomation is a library row plus the task's toggle state for harness injection.
type ResolvedAutomation struct {
	AutomationID string
	Title        string
	Description  string
	State        AutomationState
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidateAutomationFields checks title and description for create/patch.
func ValidateAutomationFields(title, description string) (string, string, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return "", "", fmt.Errorf("%w: automation title required", ErrInvalidInput)
	}
	if len(title) > maxAutomationTitleLen {
		return "", "", fmt.Errorf("%w: automation title too long", ErrInvalidInput)
	}
	if description == "" {
		return "", "", fmt.Errorf("%w: automation description required", ErrInvalidInput)
	}
	if len(description) > maxAutomationDescriptionLen {
		return "", "", fmt.Errorf("%w: automation description too long", ErrInvalidInput)
	}
	lower := strings.ToLower(description)
	if strings.HasPrefix(lower, "do not ") || strings.HasPrefix(lower, "don't ") {
		return "", "", fmt.Errorf("%w: description must state the behavior affirmatively; the harness prefixes prohibitions for no-state toggles", ErrInvalidInput)
	}
	return title, description, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NormalizeAutomationState parses and validates a toggle state.
func NormalizeAutomationState(raw AutomationState) (AutomationState, error) {
	switch AutomationState(strings.TrimSpace(string(raw))) {
	case AutomationStateYes:
		return AutomationStateYes, nil
	case AutomationStateNo:
		return AutomationStateNo, nil
	default:
		return "", fmt.Errorf("%w: automation state must be yes or no", ErrInvalidInput)
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidateAutomationSelections normalizes and validates task automation bindings.
func ValidateAutomationSelections(in []AutomationSelection) ([]AutomationSelection, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > MaxAutomationSelectionsPerTask {
		return nil, fmt.Errorf("%w: at most %d automation selections per task", ErrInvalidInput, MaxAutomationSelectionsPerTask)
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]AutomationSelection, 0, len(in))
	for _, sel := range in {
		id := strings.TrimSpace(sel.AutomationID)
		if id == "" {
			return nil, fmt.Errorf("%w: automation_id required", ErrInvalidInput)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("%w: duplicate automation_id %q", ErrInvalidInput, id)
		}
		state, err := NormalizeAutomationState(sel.State)
		if err != nil {
			return nil, err
		}
		seen[id] = struct{}{}
		out = append(out, AutomationSelection{AutomationID: id, State: state})
	}
	return out, nil
}
