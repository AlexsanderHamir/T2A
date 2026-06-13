package checklist

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// itemsForDefinitionInTx returns the canonical-ordered definition rows
// for the task that owns them (must already be the resolved definition
// owner; not the inherit-true subject).
func itemsForDefinitionInTx(tx *gorm.DB, defTaskID string) ([]domain.TaskChecklistItem, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.itemsForDefinitionInTx")
	var items []domain.TaskChecklistItem
	if err := tx.Where("task_id = ?", defTaskID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// validateChecklistCompleteInTx asserts that every definition item
// inherited by subjectTaskID has a matching task_checklist_completions
// row for the same subject task. Empty checklist == OK. Surfaces
// ErrInvalidInput when at least one item is unchecked, so the caller
// can surface a 400 to the API client.
func validateChecklistCompleteInTx(tx *gorm.DB, subjectTaskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.validateChecklistCompleteInTx")
	defID, err := DefinitionSourceTaskIDInTx(tx, subjectTaskID)
	if err != nil {
		return err
	}
	items, err := itemsForDefinitionInTx(tx, defID)
	if err != nil {
		return fmt.Errorf("checklist: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	for _, it := range items {
		var comp domain.TaskChecklistCompletion
		err := tx.Where("task_id = ? AND item_id = ?", subjectTaskID, it.ID).First(&comp).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: all checklist items must be done before marking this task done", domain.ErrInvalidInput)
		}
		if err != nil {
			return fmt.Errorf("checklist completion: %w", err)
		}
		if !domain.ValidVerifierKind(comp.VerifiedBy) {
			return fmt.Errorf("%w: checklist completion missing verified_by", domain.ErrInvalidInput)
		}
		if comp.VerifiedBy != domain.VerifierLegacy && strings.TrimSpace(comp.Evidence) == "" {
			return fmt.Errorf("%w: checklist completion missing evidence", domain.ErrInvalidInput)
		}
	}
	return nil
}

// ValidateCanMarkDoneInTx is the cross-domain guard that the task
// CRUD/update/devmirror code calls before transitioning a task to
// status=done. Requires checklist complete.
func ValidateCanMarkDoneInTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateCanMarkDoneInTx")
	return validateChecklistCompleteInTx(tx, taskID)
}

// ValidateCanAddCriterionInTx rejects appending definition rows once the
// agent has picked up the task or it has reached a terminal done state.
func ValidateCanAddCriterionInTx(tx *gorm.DB, t *domain.Task) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateCanAddCriterionInTx")
	if t == nil {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	switch t.Status {
	case domain.StatusRunning, domain.StatusDone:
		return fmt.Errorf("%w: cannot add criteria while task is %s", domain.ErrConflict, t.Status)
	default:
		return nil
	}
}

// DeleteOwnedItemsInTx removes every checklist definition row owned
// by taskID and the per-subject completion rows that point at those
// items. Exported so the task update/delete paths can drop a task's
// checklist atomically alongside the parent row.
func DeleteOwnedItemsInTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.DeleteOwnedItemsInTx")
	var ids []string
	if err := tx.Model(&domain.TaskChecklistItem{}).Where("task_id = ?", taskID).Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("list checklist items: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Where("item_id IN ?", ids).Delete(&domain.TaskChecklistCompletion{}).Error; err != nil {
		return fmt.Errorf("delete completions: %w", err)
	}
	if err := tx.Where("task_id = ?", taskID).Delete(&domain.TaskChecklistItem{}).Error; err != nil {
		return fmt.Errorf("delete checklist items: %w", err)
	}
	return nil
}
