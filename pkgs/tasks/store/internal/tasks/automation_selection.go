package tasks

import (
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/automations"
	"gorm.io/gorm"
)

func normalizeAutomationSelections(raw []domain.AutomationSelection) ([]domain.AutomationSelection, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.normalizeAutomationSelections")
	return domain.ValidateAutomationSelections(raw)
}

func applyAutomationSelectionsPatch(tx *gorm.DB, cur *domain.Task, selections *[]domain.AutomationSelection) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.applyAutomationSelectionsPatch")
	if selections == nil {
		return nil
	}
	normalized, err := normalizeAutomationSelections(*selections)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		cur.AutomationSelections = nil
		return nil
	}
	if err := automations.ValidateSelectionIDs(tx.Statement.Context, tx, normalized); err != nil {
		return err
	}
	cur.AutomationSelections = normalized
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func applyAutomationSelectionsOnCreate(tx *gorm.DB, t *domain.Task, raw []domain.AutomationSelection) error {
	normalized, err := normalizeAutomationSelections(raw)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		t.AutomationSelections = nil
		return nil
	}
	if err := automations.ValidateSelectionIDs(tx.Statement.Context, tx, normalized); err != nil {
		return err
	}
	t.AutomationSelections = normalized
	return nil
}
