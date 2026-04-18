package checklist

import (
	"fmt"
	"log/slog"

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
		var n int64
		if err := tx.Model(&domain.TaskChecklistCompletion{}).
			Where("task_id = ? AND item_id = ?", subjectTaskID, it.ID).
			Count(&n).Error; err != nil {
			return fmt.Errorf("checklist completion: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: all checklist items must be done before marking this task done", domain.ErrInvalidInput)
		}
	}
	return nil
}

// validateDescendantsDoneInTx walks the subtree rooted at taskID and
// rejects when any descendant is not status=done. Used by
// ValidateCanMarkDoneInTx so the "done" rollup honors the parent /
// children invariant.
func validateDescendantsDoneInTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.validateDescendantsDoneInTx")
	queue := []string{taskID}
	for len(queue) > 0 {
		var children []domain.Task
		if err := tx.Where("parent_id IN ?", queue).Find(&children).Error; err != nil {
			return fmt.Errorf("list children: %w", err)
		}
		queue = queue[:0]
		for _, c := range children {
			if c.Status != domain.StatusDone {
				return fmt.Errorf("%w: all subtasks must be done before marking this task done", domain.ErrInvalidInput)
			}
			queue = append(queue, c.ID)
		}
	}
	return nil
}

// ValidateCanMarkDoneInTx is the cross-domain guard that the task
// CRUD/update/devmirror code calls before transitioning a task to
// status=done. It enforces both invariants in a single transactional
// hop:
//
//  1. every descendant task is already StatusDone, and
//  2. every checklist item inherited by the task has a matching
//     completion row.
//
// Exported so subpackages outside the checklist domain can compose
// the "mark done" transaction without reaching into private helpers.
// Returns ErrInvalidInput when either invariant fails so handlers
// can surface a 400.
func ValidateCanMarkDoneInTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateCanMarkDoneInTx")
	if err := validateDescendantsDoneInTx(tx, taskID); err != nil {
		return err
	}
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
