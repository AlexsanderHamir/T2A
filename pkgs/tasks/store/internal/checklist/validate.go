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

// parentOwnedChecklistCountInTx counts definition rows owned directly by taskID.
func parentOwnedChecklistCountInTx(tx *gorm.DB, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.parentOwnedChecklistCountInTx")
	var n int64
	if err := tx.Model(&domain.TaskChecklistItem{}).Where("task_id = ?", taskID).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count checklist items: %w", err)
	}
	return n, nil
}

// parentHasSubtasksInTx reports whether any task lists parentID as parent_id.
func parentHasSubtasksInTx(tx *gorm.DB, parentID string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.parentHasSubtasksInTx")
	var n int64
	if err := tx.Model(&domain.Task{}).Where("parent_id = ?", parentID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("count subtasks: %w", err)
	}
	return n > 0, nil
}

// ValidateParentCanHaveSubtasksInTx requires a root parent to define at
// least one done criterion before subtasks are linked. Subtasks use
// depends_on + status=done on the parent; criteria give the parent an
// explicit, verify-backed completion signal.
func ValidateParentCanHaveSubtasksInTx(tx *gorm.DB, parentID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateParentCanHaveSubtasksInTx")
	var parent domain.Task
	if err := tx.Where("id = ?", parentID).First(&parent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: parent not found", domain.ErrInvalidInput)
		}
		return fmt.Errorf("load parent: %w", err)
	}
	if parent.ParentID != nil && strings.TrimSpace(*parent.ParentID) != "" {
		return nil
	}
	n, err := parentOwnedChecklistCountInTx(tx, parentID)
	if err != nil {
		return err
	}
	if n < 1 {
		return fmt.Errorf("%w: parent task with subtasks requires at least one done criterion", domain.ErrInvalidInput)
	}
	return nil
}

// ValidateParentCanRemoveLastCriterionInTx rejects removing the final
// owned checklist item while subtasks still exist on a root parent.
func ValidateParentCanRemoveLastCriterionInTx(tx *gorm.DB, parentID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateParentCanRemoveLastCriterionInTx")
	hasSubtasks, err := parentHasSubtasksInTx(tx, parentID)
	if err != nil {
		return err
	}
	if !hasSubtasks {
		return nil
	}
	var parent domain.Task
	if err := tx.Where("id = ?", parentID).First(&parent).Error; err != nil {
		return fmt.Errorf("load parent: %w", err)
	}
	if parent.ParentID != nil && strings.TrimSpace(*parent.ParentID) != "" {
		return nil
	}
	n, err := parentOwnedChecklistCountInTx(tx, parentID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return fmt.Errorf("%w: parent task with subtasks requires at least one done criterion", domain.ErrInvalidInput)
	}
	return nil
}

// ValidateCanMarkDoneInTx is the cross-domain guard that the task
// CRUD/update/devmirror code calls before transitioning a task to
// status=done. Parent completion is criteria-driven: every checklist
// item inherited by the task must have a matching completion row.
// Subtask completion is independent unless explicitly linked via depends_on.
//
// Exported so subpackages outside the checklist domain can compose
// the "mark done" transaction without reaching into private helpers.
// Returns ErrInvalidInput when checklist is incomplete so handlers
// can surface a 400.
func ValidateCanMarkDoneInTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateCanMarkDoneInTx")
	return validateChecklistCompleteInTx(tx, taskID)
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
