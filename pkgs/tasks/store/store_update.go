package store

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func wouldCreateParentCycle(tx *gorm.DB, taskID, newParent string) (bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.wouldCreateParentCycle")
	cur := strings.TrimSpace(newParent)
	seen := make(map[string]bool)
	for cur != "" {
		if cur == taskID {
			return true, nil
		}
		if seen[cur] {
			return true, fmt.Errorf("%w: parent cycle", domain.ErrInvalidInput)
		}
		seen[cur] = true
		var t domain.Task
		if err := tx.Where("id = ?", cur).First(&t).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, domain.ErrNotFound
			}
			return false, fmt.Errorf("load parent chain: %w", err)
		}
		if t.ParentID == nil || *t.ParentID == "" {
			break
		}
		cur = *t.ParentID
	}
	return false, nil
}

func applyTaskPatches(tx *gorm.DB, taskID string, cur *domain.Task, in UpdateTaskInput, by domain.Actor, seq int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyTaskPatches")
	seqPtr := seq
	if err := applyTitlePatch(tx, taskID, cur, in.Title, by, &seqPtr); err != nil {
		return err
	}
	if err := applyInitialPromptPatch(tx, taskID, cur, in.InitialPrompt, by, &seqPtr); err != nil {
		return err
	}
	if err := applyParentPatch(tx, taskID, cur, in.Parent, by, &seqPtr); err != nil {
		return err
	}
	if err := applyChecklistInheritPatch(tx, taskID, cur, in.ChecklistInherit, by, &seqPtr); err != nil {
		return err
	}
	if err := applyPriorityPatch(tx, taskID, cur, in.Priority, by, &seqPtr); err != nil {
		return err
	}
	if err := applyTaskTypePatch(cur, in.TaskType); err != nil {
		return err
	}
	if err := applyStatusPatch(tx, taskID, cur, in.Status, by, &seqPtr); err != nil {
		return err
	}
	if cur.ChecklistInherit && (cur.ParentID == nil || *cur.ParentID == "") {
		return fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}
	return nil
}
