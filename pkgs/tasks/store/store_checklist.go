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
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChecklistItemView is one definition row plus completion for a subject task.
type ChecklistItemView struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
}

// DefinitionSourceTaskID returns the task id that owns checklist item definitions for id.
func (s *Store) DefinitionSourceTaskID(ctx context.Context, taskID string) (string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DefinitionSourceTaskID")
	return definitionSourceTaskIDTx(s.db.WithContext(ctx), taskID)
}

func definitionSourceTaskIDTx(tx *gorm.DB, taskID string) (string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.definitionSourceTaskIDTx")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	cur := taskID
	seen := make(map[string]bool)
	for {
		if seen[cur] {
			return "", fmt.Errorf("%w: parent cycle", domain.ErrInvalidInput)
		}
		seen[cur] = true
		var t domain.Task
		if err := tx.Where("id = ?", cur).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", domain.ErrNotFound
			}
			return "", fmt.Errorf("load task: %w", err)
		}
		if !t.ChecklistInherit {
			return t.ID, nil
		}
		if t.ParentID == nil || *t.ParentID == "" {
			return "", fmt.Errorf("%w: checklist_inherit requires a parent task", domain.ErrInvalidInput)
		}
		cur = *t.ParentID
	}
}

// ListChecklistForSubject returns definition items for taskID with done flags for that same task.
func (s *Store) ListChecklistForSubject(ctx context.Context, taskID string) ([]ChecklistItemView, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListChecklistForSubject")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var out []ChecklistItemView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := txLoadTask(tx, taskID); err != nil {
			return err
		}
		defID, err := definitionSourceTaskIDTx(tx, taskID)
		if err != nil {
			return err
		}
		var items []domain.TaskChecklistItem
		if err := tx.Where("task_id = ?", defID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
			return fmt.Errorf("list checklist items: %w", err)
		}
		if len(items) == 0 {
			out = []ChecklistItemView{}
			return nil
		}
		ids := make([]string, len(items))
		for i := range items {
			ids[i] = items[i].ID
		}
		var doneRows []domain.TaskChecklistCompletion
		if err := tx.Where("task_id = ? AND item_id IN ?", taskID, ids).Find(&doneRows).Error; err != nil {
			return fmt.Errorf("list checklist completions: %w", err)
		}
		doneSet := make(map[string]bool, len(doneRows))
		for _, d := range doneRows {
			doneSet[d.ItemID] = true
		}
		out = make([]ChecklistItemView, 0, len(items))
		for _, it := range items {
			out = append(out, ChecklistItemView{
				ID:        it.ID,
				SortOrder: it.SortOrder,
				Text:      it.Text,
				Done:      doneSet[it.ID],
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func txLoadTask(tx *gorm.DB, id string) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.txLoadTask")
	var t domain.Task
	if err := tx.Where("id = ?", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("load task: %w", err)
	}
	return &t, nil
}

// AddChecklistItem appends a definition row; task must exist and not use checklist_inherit.
func (s *Store) AddChecklistItem(ctx context.Context, taskID, text string, by domain.Actor) (*domain.TaskChecklistItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AddChecklistItem")
	if err := validateActor(by); err != nil {
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
		t, err := txLoadTask(tx, taskID)
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
		seq, err := nextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]string{"item_id": it.ID, "text": it.Text})
		if err := appendEvent(tx, taskID, seq, domain.EventChecklistItemAdded, by, b); err != nil {
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
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteChecklistItem")
	if err := validateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	if taskID == "" || itemID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := txLoadTask(tx, taskID)
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
		seq, err := nextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]string{"item_id": itemID, "text": it.Text})
		if err := appendEvent(tx, taskID, seq, domain.EventChecklistItemRemoved, by, b); err != nil {
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
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateChecklistItemText")
	if err := validateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	text = strings.TrimSpace(text)
	if taskID == "" || itemID == "" || text == "" {
		return fmt.Errorf("%w: text", domain.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := txLoadTask(tx, taskID)
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
		seq, err := nextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{"item_id": itemID, "text": text})
		return appendEvent(tx, taskID, seq, domain.EventChecklistItemUpdated, by, b)
	})
}

// SetChecklistItemDone sets or clears completion for subjectTaskID on an item from its definition source.
// Only [domain.ActorAgent] may change completion; the human user records criteria (POST) but does not toggle done.
func (s *Store) SetChecklistItemDone(ctx context.Context, subjectTaskID, itemID string, done bool, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetChecklistItemDone")
	if err := validateActor(by); err != nil {
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
		if _, err := txLoadTask(tx, subjectTaskID); err != nil {
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
		seq, err := nextEventSeq(tx, subjectTaskID)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{"item_id": itemID, "done": done})
		if err := appendEvent(tx, subjectTaskID, seq, domain.EventChecklistItemToggled, by, b); err != nil {
			return err
		}
		return nil
	})
}

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
