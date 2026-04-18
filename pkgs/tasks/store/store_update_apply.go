package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

func applyTitlePatch(tx *gorm.DB, taskID string, cur *domain.Task, title *string, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyTitlePatch")
	if title == nil {
		return nil
	}
	v := strings.TrimSpace(*title)
	if v == "" {
		return fmt.Errorf("%w: title", domain.ErrInvalidInput)
	}
	if v == cur.Title {
		return nil
	}
	b, err := kernel.EventPairJSON(cur.Title, v)
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventMessageAdded, by, b); err != nil {
		return err
	}
	*seq++
	cur.Title = v
	return nil
}

func applyInitialPromptPatch(tx *gorm.DB, taskID string, cur *domain.Task, prompt *string, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyInitialPromptPatch")
	if prompt == nil {
		return nil
	}
	if *prompt == cur.InitialPrompt {
		return nil
	}
	b, err := kernel.EventPairJSON(cur.InitialPrompt, *prompt)
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventPromptAppended, by, b); err != nil {
		return err
	}
	*seq++
	cur.InitialPrompt = *prompt
	return nil
}

func applyParentPatch(tx *gorm.DB, taskID string, cur *domain.Task, parent *ParentFieldPatch, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyParentPatch")
	if parent == nil {
		return nil
	}
	var prevStr string
	if cur.ParentID != nil {
		prevStr = *cur.ParentID
	}
	var nextStr string
	var nextPtr *string
	if parent.Clear {
		nextPtr = nil
	} else {
		pid := strings.TrimSpace(parent.ID)
		if pid == "" {
			return fmt.Errorf("%w: parent_id", domain.ErrInvalidInput)
		}
		if pid == taskID {
			return fmt.Errorf("%w: task cannot be its own parent", domain.ErrInvalidInput)
		}
		var n int64
		if err := tx.Model(&domain.Task{}).Where("id = ?", pid).Count(&n).Error; err != nil {
			return fmt.Errorf("parent lookup: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: parent not found", domain.ErrInvalidInput)
		}
		cycle, err := wouldCreateParentCycle(tx, taskID, pid)
		if err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("%w: parent would create a cycle", domain.ErrInvalidInput)
		}
		nextPtr = &pid
		nextStr = pid
	}
	if prevStr != nextStr {
		b, err := json.Marshal(map[string]string{
			"parent_id":          nextStr,
			"previous_parent_id": prevStr,
		})
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventSubtaskAdded, by, b); err != nil {
			return err
		}
		*seq++
	}
	cur.ParentID = nextPtr
	return nil
}

func applyChecklistInheritPatch(tx *gorm.DB, taskID string, cur *domain.Task, inherit *bool, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyChecklistInheritPatch")
	if inherit == nil {
		return nil
	}
	was := cur.ChecklistInherit
	want := *inherit
	if want && !was {
		if err := checklist.DeleteOwnedItemsInTx(tx, taskID); err != nil {
			return err
		}
	}
	if want != was {
		b, err := json.Marshal(map[string]bool{"from": was, "to": want})
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventChecklistInheritChanged, by, b); err != nil {
			return err
		}
		*seq++
	}
	cur.ChecklistInherit = want
	return nil
}

func applyPriorityPatch(tx *gorm.DB, taskID string, cur *domain.Task, pr *domain.Priority, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyPriorityPatch")
	if pr == nil {
		return nil
	}
	if !kernel.ValidPriority(*pr) {
		return fmt.Errorf("%w: priority", domain.ErrInvalidInput)
	}
	if *pr == cur.Priority {
		return nil
	}
	b, err := kernel.EventPairJSON(string(cur.Priority), string(*pr))
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventPriorityChanged, by, b); err != nil {
		return err
	}
	*seq++
	cur.Priority = *pr
	return nil
}

func applyTaskTypePatch(cur *domain.Task, tt *domain.TaskType) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyTaskTypePatch")
	if tt == nil {
		return nil
	}
	if !kernel.ValidTaskType(*tt) {
		return fmt.Errorf("%w: task_type", domain.ErrInvalidInput)
	}
	cur.TaskType = *tt
	return nil
}

func applyStatusPatch(tx *gorm.DB, taskID string, cur *domain.Task, st *domain.Status, by domain.Actor, seq *int64) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.applyStatusPatch")
	if st == nil {
		return nil
	}
	if !kernel.ValidStatus(*st) {
		return fmt.Errorf("%w: status", domain.ErrInvalidInput)
	}
	if *st == cur.Status {
		return nil
	}
	if *st == domain.StatusDone {
		if err := checklist.ValidateCanMarkDoneInTx(tx, taskID); err != nil {
			return err
		}
	}
	b, err := kernel.EventPairJSON(string(cur.Status), string(*st))
	if err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, taskID, *seq, domain.EventStatusChanged, by, b); err != nil {
		return err
	}
	*seq++
	cur.Status = *st
	return nil
}
