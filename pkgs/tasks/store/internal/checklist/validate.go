package checklist

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
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

// ValidateCanAddCriterionInTx rejects appending definition rows while the
// agent is actively working the task (status=running).
func ValidateCanAddCriterionInTx(tx *gorm.DB, t *domain.Task) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ValidateCanAddCriterionInTx")
	return validateCriteriaMutable(t)
}

// ValidateCriteriaMutable rejects user-driven checklist mutations while the
// task is in progress. Done tasks remain editable for post-completion tweaks.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ValidateCriteriaMutable(t *domain.Task) error {
	return validateCriteriaMutable(t)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func validateCriteriaMutable(t *domain.Task) error {
	if t == nil {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if t.Status == domain.StatusRunning {
		return fmt.Errorf("%w: cannot change criteria while task is running", domain.ErrConflict)
	}
	return nil
}

// criterionLockedByCompletion reports whether existing completion rows block
// definition edits. Satisfied criteria stay locked while the task is still
// active; once status=done operators may revise definitions.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func criterionLockedByCompletion(taskStatus domain.Status, doneCount int64) bool {
	if doneCount == 0 {
		return false
	}
	return taskStatus != domain.StatusDone
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
