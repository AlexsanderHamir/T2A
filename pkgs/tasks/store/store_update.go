package store

import (
	"encoding/json"
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
	if in.Title != nil {
		v := strings.TrimSpace(*in.Title)
		if v == "" {
			return fmt.Errorf("%w: title", domain.ErrInvalidInput)
		}
		if v != cur.Title {
			b, err := eventPairJSON(cur.Title, v)
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventMessageAdded, by, b); err != nil {
				return err
			}
			seq++
			cur.Title = v
		}
	}
	if in.InitialPrompt != nil {
		if *in.InitialPrompt != cur.InitialPrompt {
			b, err := eventPairJSON(cur.InitialPrompt, *in.InitialPrompt)
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventPromptAppended, by, b); err != nil {
				return err
			}
			seq++
			cur.InitialPrompt = *in.InitialPrompt
		}
	}
	if in.Parent != nil {
		var prevStr string
		if cur.ParentID != nil {
			prevStr = *cur.ParentID
		}
		var nextStr string
		var nextPtr *string
		if in.Parent.Clear {
			nextPtr = nil
		} else {
			pid := strings.TrimSpace(in.Parent.ID)
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
			if err := appendEvent(tx, taskID, seq, domain.EventSubtaskAdded, by, b); err != nil {
				return err
			}
			seq++
		}
		cur.ParentID = nextPtr
	}
	if in.ChecklistInherit != nil {
		was := cur.ChecklistInherit
		want := *in.ChecklistInherit
		if want && !was {
			if err := deleteOwnedChecklistItemsTx(tx, taskID); err != nil {
				return err
			}
		}
		if want != was {
			b, err := json.Marshal(map[string]bool{"from": was, "to": want})
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventChecklistInheritChanged, by, b); err != nil {
				return err
			}
			seq++
		}
		cur.ChecklistInherit = want
	}
	if in.Priority != nil {
		if !validPriority(*in.Priority) {
			return fmt.Errorf("%w: priority", domain.ErrInvalidInput)
		}
		if *in.Priority != cur.Priority {
			b, err := eventPairJSON(string(cur.Priority), string(*in.Priority))
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventPriorityChanged, by, b); err != nil {
				return err
			}
			seq++
			cur.Priority = *in.Priority
		}
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return fmt.Errorf("%w: status", domain.ErrInvalidInput)
		}
		if *in.Status != cur.Status {
			if *in.Status == domain.StatusDone {
				if err := validateCanMarkDoneTx(tx, taskID); err != nil {
					return err
				}
			}
			b, err := eventPairJSON(string(cur.Status), string(*in.Status))
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventStatusChanged, by, b); err != nil {
				return err
			}
			cur.Status = *in.Status
		}
	}
	if cur.ChecklistInherit && (cur.ParentID == nil || *cur.ParentID == "") {
		return fmt.Errorf("%w: checklist_inherit requires parent_id", domain.ErrInvalidInput)
	}
	return nil
}
