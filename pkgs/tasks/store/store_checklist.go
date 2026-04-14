package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
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
	defer deferStoreLatency(storeOpDefinitionSourceTask)()
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
	defer deferStoreLatency(storeOpListChecklist)()
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
