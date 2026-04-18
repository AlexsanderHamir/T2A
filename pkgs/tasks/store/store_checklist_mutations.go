package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AddChecklistItem appends a definition row; task must exist and not use checklist_inherit.
func (s *Store) AddChecklistItem(ctx context.Context, taskID, text string, by domain.Actor) (*domain.TaskChecklistItem, error) {
	defer kernel.DeferLatency(kernel.OpAddChecklistItem)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AddChecklistItem")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: checklist text required", domain.ErrInvalidInput)
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var created *domain.TaskChecklistItem
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := kernel.LoadTask(tx, taskID)
		if err != nil {
			return err
		}
		if t.ChecklistInherit {
			return fmt.Errorf("%w: cannot add checklist items while checklist_inherit is true", domain.ErrInvalidInput)
		}
		var maxOrder int
		row := tx.Model(&domain.TaskChecklistItem{}).Select("COALESCE(MAX(sort_order), 0)").Where("task_id = ?", taskID)
		if err := row.Scan(&maxOrder).Error; err != nil {
			return fmt.Errorf("checklist order: %w", err)
		}
		it := &domain.TaskChecklistItem{
			ID:        uuid.NewString(),
			TaskID:    taskID,
			SortOrder: maxOrder + 1,
			Text:      text,
		}
		if err := tx.Create(it).Error; err != nil {
			return fmt.Errorf("insert checklist item: %w", err)
		}
		seq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]string{"item_id": it.ID, "text": it.Text})
		if err := kernel.AppendEvent(tx, taskID, seq, domain.EventChecklistItemAdded, by, b); err != nil {
			return err
		}
		created = it
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("add checklist item: %w", err)
	}
	return created, nil
}

// DeleteChecklistItem removes a definition row owned by taskID.
func (s *Store) DeleteChecklistItem(ctx context.Context, taskID, itemID string, by domain.Actor) error {
	defer kernel.DeferLatency(kernel.OpDeleteChecklistItem)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteChecklistItem")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	if taskID == "" || itemID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := kernel.LoadTask(tx, taskID)
		if err != nil {
			return err
		}
		if t.ChecklistInherit {
			return fmt.Errorf("%w: cannot delete inherited checklist definitions from this task", domain.ErrInvalidInput)
		}
		var it domain.TaskChecklistItem
		if err := tx.Where("id = ? AND task_id = ?", itemID, taskID).First(&it).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load checklist item: %w", err)
		}
		if err := tx.Where("item_id = ?", itemID).Delete(&domain.TaskChecklistCompletion{}).Error; err != nil {
			return fmt.Errorf("delete completions: %w", err)
		}
		seq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]string{"item_id": itemID, "text": it.Text})
		if err := kernel.AppendEvent(tx, taskID, seq, domain.EventChecklistItemRemoved, by, b); err != nil {
			return err
		}
		if err := tx.Delete(&it).Error; err != nil {
			return fmt.Errorf("delete checklist item: %w", err)
		}
		return nil
	})
}

// UpdateChecklistItemText updates the definition text for an item owned by taskID.
// Rejected when the task uses checklist_inherit or the item is not on that task.
func (s *Store) UpdateChecklistItemText(ctx context.Context, taskID, itemID, text string, by domain.Actor) error {
	defer kernel.DeferLatency(kernel.OpUpdateChecklistItemText)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateChecklistItemText")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	text = strings.TrimSpace(text)
	if taskID == "" || itemID == "" || text == "" {
		return fmt.Errorf("%w: text", domain.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := kernel.LoadTask(tx, taskID)
		if err != nil {
			return err
		}
		if t.ChecklistInherit {
			return fmt.Errorf("%w: cannot update inherited checklist definitions from this task", domain.ErrInvalidInput)
		}
		var it domain.TaskChecklistItem
		if err := tx.Where("id = ? AND task_id = ?", itemID, taskID).First(&it).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load checklist item: %w", err)
		}
		if it.Text == text {
			return nil
		}
		if err := tx.Model(&it).Update("text", text).Error; err != nil {
			return fmt.Errorf("update checklist item: %w", err)
		}
		seq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{"item_id": itemID, "text": text})
		return kernel.AppendEvent(tx, taskID, seq, domain.EventChecklistItemUpdated, by, b)
	})
}

// SetChecklistItemDone sets or clears completion for subjectTaskID on an item from its definition source.
// Only [domain.ActorAgent] may change completion; the human user records criteria (POST) but does not toggle done.
func (s *Store) SetChecklistItemDone(ctx context.Context, subjectTaskID, itemID string, done bool, by domain.Actor) error {
	defer kernel.DeferLatency(kernel.OpSetChecklistItemDone)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetChecklistItemDone")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	if by != domain.ActorAgent {
		return fmt.Errorf("%w: only the agent may mark checklist items done or undone", domain.ErrInvalidInput)
	}
	subjectTaskID = strings.TrimSpace(subjectTaskID)
	itemID = strings.TrimSpace(itemID)
	if subjectTaskID == "" || itemID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := kernel.LoadTask(tx, subjectTaskID); err != nil {
			return err
		}
		defOwner, err := definitionSourceTaskIDTx(tx, subjectTaskID)
		if err != nil {
			return err
		}
		var it domain.TaskChecklistItem
		if err := tx.Where("id = ? AND task_id = ?", itemID, defOwner).First(&it).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load checklist item: %w", err)
		}
		if done {
			row := domain.TaskChecklistCompletion{
				TaskID: subjectTaskID,
				ItemID: itemID,
				At:     time.Now().UTC(),
				By:     by,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "task_id"}, {Name: "item_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"at", "done_by"}),
			}).Create(&row).Error; err != nil {
				return fmt.Errorf("save completion: %w", err)
			}
		} else {
			res := tx.Where("task_id = ? AND item_id = ?", subjectTaskID, itemID).Delete(&domain.TaskChecklistCompletion{})
			if res.Error != nil {
				return fmt.Errorf("delete completion: %w", res.Error)
			}
		}
		seq, err := kernel.NextEventSeq(tx, subjectTaskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{"item_id": itemID, "done": done})
		if err := kernel.AppendEvent(tx, subjectTaskID, seq, domain.EventChecklistItemToggled, by, b); err != nil {
			return err
		}
		return nil
	})
}
