package checklist

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

const logCmd = "taskapi"

// ItemView is one definition row plus completion for a subject task.
// Re-aliased by the store facade as store.ChecklistItemView so the
// JSON field tags stay stable on the wire.
type ItemView struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
}

// DefinitionSourceTaskID returns the task id that owns checklist item
// definitions for taskID. Walks the ParentID chain through any
// ChecklistInherit-true ancestors. Errors:
//   - ErrNotFound when the task or an ancestor is missing.
//   - ErrInvalidInput when an inherit-true task has no parent, or a
//     cycle in the parent chain is detected.
func DefinitionSourceTaskID(ctx context.Context, db *gorm.DB, taskID string) (string, error) {
	defer kernel.DeferLatency(kernel.OpDefinitionSourceTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.DefinitionSourceTaskID")
	return DefinitionSourceTaskIDInTx(db.WithContext(ctx), taskID)
}

// DefinitionSourceTaskIDInTx is the in-transaction variant used by
// other internal store packages that already hold a *gorm.DB tx
// handle.
func DefinitionSourceTaskIDInTx(tx *gorm.DB, taskID string) (string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.DefinitionSourceTaskIDInTx")
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

// List returns definition items for taskID with done flags for that
// same task. The taskID must exist; otherwise ErrNotFound.
func List(ctx context.Context, db *gorm.DB, taskID string) ([]ItemView, error) {
	defer kernel.DeferLatency(kernel.OpListChecklist)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.List")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var out []ItemView
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := kernel.LoadTask(tx, taskID); err != nil {
			return err
		}
		defID, err := DefinitionSourceTaskIDInTx(tx, taskID)
		if err != nil {
			return err
		}
		items, err := itemsForDefinitionInTx(tx, defID)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			out = []ItemView{}
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
		out = make([]ItemView, 0, len(items))
		for _, it := range items {
			out = append(out, ItemView{
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

// Add appends a definition row; the task must exist and not use
// ChecklistInherit. Appends EventChecklistItemAdded in the same TX.
func Add(ctx context.Context, db *gorm.DB, taskID, text string, by domain.Actor) (*domain.TaskChecklistItem, error) {
	defer kernel.DeferLatency(kernel.OpAddChecklistItem)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.Add")
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
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// Delete removes a definition row owned by taskID. Cascades to the
// per-subject completion rows for that item. Appends
// EventChecklistItemRemoved in the same TX.
func Delete(ctx context.Context, db *gorm.DB, taskID, itemID string, by domain.Actor) error {
	defer kernel.DeferLatency(kernel.OpDeleteChecklistItem)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.Delete")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	if taskID == "" || itemID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// UpdateText updates the definition text for an item owned by taskID.
// No-op (no event emitted) when the new text matches the existing
// row, so idempotent UI saves do not pollute the audit log. Appends
// EventChecklistItemUpdated in the same TX otherwise.
func UpdateText(ctx context.Context, db *gorm.DB, taskID, itemID, text string, by domain.Actor) error {
	defer kernel.DeferLatency(kernel.OpUpdateChecklistItemText)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.UpdateText")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	text = strings.TrimSpace(text)
	if taskID == "" || itemID == "" || text == "" {
		return fmt.Errorf("%w: text", domain.ErrInvalidInput)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// SetDone sets or clears completion for subjectTaskID on an item
// resolved through DefinitionSourceTaskIDInTx. Only domain.ActorAgent
// may change completion; the human user records criteria via Add but
// does not toggle done. Appends EventChecklistItemToggled in the same
// TX.
func SetDone(ctx context.Context, db *gorm.DB, subjectTaskID, itemID string, done bool, by domain.Actor) error {
	defer kernel.DeferLatency(kernel.OpSetChecklistItemDone)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.SetDone")
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
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := kernel.LoadTask(tx, subjectTaskID); err != nil {
			return err
		}
		defOwner, err := DefinitionSourceTaskIDInTx(tx, subjectTaskID)
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
		// Idempotency guard — mirror the convention established by every
		// other patch path (UpdateText above, applyTitlePatch,
		// applyInitialPromptPatch, applyChecklistInheritPatch,
		// applyPriorityPatch, applyStatusPatch): if the requested state
		// already matches the persisted state, treat the call as a no-op.
		// Skipping the write and the event keeps the audit log free of
		// noise on agent retries (a single agent run can re-assert the
		// same checklist completion many times), prevents the completion
		// `at` timestamp from being silently re-stamped on no-op
		// done=true calls, and avoids broadcasting `task_updated` SSE
		// fanouts for changes that didn't happen.
		var existing domain.TaskChecklistCompletion
		err = tx.Where("task_id = ? AND item_id = ?", subjectTaskID, itemID).First(&existing).Error
		switch {
		case err == nil:
			if done {
				// Already done — no-op.
				return nil
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if !done {
				// Already not-done — no-op.
				return nil
			}
		default:
			return fmt.Errorf("load completion: %w", err)
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
