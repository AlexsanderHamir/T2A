package store

import (
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func checklistItemsForDefinitionTx(tx *gorm.DB, defTaskID string) ([]domain.TaskChecklistItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.checklistItemsForDefinitionTx")
	var items []domain.TaskChecklistItem
	if err := tx.Where("task_id = ?", defTaskID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func validateChecklistCompleteTx(tx *gorm.DB, subjectTaskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validateChecklistCompleteTx")
	defID, err := definitionSourceTaskIDTx(tx, subjectTaskID)
	if err != nil {
		return err
	}
	items, err := checklistItemsForDefinitionTx(tx, defID)
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

func validateDescendantsDoneTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validateDescendantsDoneTx")
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

func validateCanMarkDoneTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validateCanMarkDoneTx")
	if err := validateDescendantsDoneTx(tx, taskID); err != nil {
		return err
	}
	return validateChecklistCompleteTx(tx, taskID)
}

func deleteOwnedChecklistItemsTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.deleteOwnedChecklistItemsTx")
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
