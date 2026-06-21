package checklist

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// VerifyItem is a criterion row for worker verification snapshots.
type VerifyItem struct {
	ID             string
	Text           string
	SourceTaskID   string
	DefinitionTask string
	VerifyCommands []VerifyCommandView
}

// ListForVerify returns all definition items for the subject task's
// checklist source.
func ListForVerify(ctx context.Context, db *gorm.DB, taskID string) ([]VerifyItem, error) {
	defer kernel.DeferLatency(kernel.OpListChecklist)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.ListForVerify")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var out []VerifyItem
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
		ids := make([]string, len(items))
		for i := range items {
			ids[i] = items[i].ID
		}
		cmdsByItem, err := commandsForItemsInTx(tx, ids)
		if err != nil {
			return err
		}
		out = make([]VerifyItem, 0, len(items))
		for _, it := range items {
			out = append(out, VerifyItem{
				ID:             it.ID,
				Text:           it.Text,
				SourceTaskID:   taskID,
				DefinitionTask: defID,
				VerifyCommands: cmdsByItem[it.ID],
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
