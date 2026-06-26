package checklist

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/kernel"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SeedDefinitionItemsAtCreateInTx inserts definition rows during POST /tasks
// inside the create transaction. Unlike Add, it does not re-check
// ValidateCanAddCriterionInTx because the row was just inserted.
func SeedDefinitionItemsAtCreateInTx(tx *gorm.DB, taskID string, items []CreateChecklistItemInput, by domain.Actor) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.checklist.SeedDefinitionItemsAtCreateInTx")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if len(items) == 0 {
		return nil
	}
	if _, err := kernel.LoadTask(tx, taskID); err != nil {
		return err
	}
	var maxOrder int
	row := tx.Model(&model.TaskChecklistItem{}).Select("COALESCE(MAX(sort_order), 0)").Where("task_id = ?", taskID)
	if err := row.Scan(&maxOrder).Error; err != nil {
		return fmt.Errorf("checklist order: %w", err)
	}
	seq, err := kernel.NextEventSeq(tx, taskID)
	if err != nil {
		return err
	}
	for _, raw := range items {
		text := strings.TrimSpace(raw.Text)
		if text == "" {
			continue
		}
		cmds, err := NormalizeVerifyCommandInputs(raw.VerifyCommands)
		if err != nil {
			return err
		}
		maxOrder++
		it := domain.TaskChecklistItem{
			ID:        uuid.NewString(),
			TaskID:    taskID,
			SortOrder: maxOrder,
			Text:      text,
		}
		if err := tx.Create(model.FromDomainTaskChecklistItemPtr(&it)).Error; err != nil {
			return fmt.Errorf("insert checklist item: %w", err)
		}
		if len(cmds) > 0 {
			if err := replaceCommandsInTx(tx, it.ID, cmds); err != nil {
				return err
			}
		}
		b, _ := json.Marshal(map[string]string{"item_id": it.ID, "text": it.Text})
		if err := kernel.AppendEvent(tx, taskID, seq, domain.EventChecklistItemAdded, by, b); err != nil {
			return err
		}
		seq++
	}
	return nil
}
