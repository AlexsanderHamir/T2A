package checklist

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/kernel"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateChecklistItemInput is one criterion seeded at task create.
type CreateChecklistItemInput struct {
	Text           string               `json:"text"`
	VerifyCommands []VerifyCommandInput `json:"verify_commands,omitempty"`
}
type VerifyCommandInput struct {
	Command         string `json:"command"`
	ExpectedOutcome string `json:"expected_outcome,omitempty"`
}

// VerifyCommandView is a persisted command row on checklist API responses.
type VerifyCommandView struct {
	SortOrder       int    `json:"sort_order"`
	Command         string `json:"command"`
	ExpectedOutcome string `json:"expected_outcome,omitempty"`
}

// NormalizeVerifyCommandInputs trims, drops blank commands, and validates limits.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func NormalizeVerifyCommandInputs(in []VerifyCommandInput) ([]VerifyCommandInput, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]VerifyCommandInput, 0, len(in))
	for _, raw := range in {
		cmd := strings.TrimSpace(raw.Command)
		if cmd == "" {
			continue
		}
		if len(cmd) > domain.MaxVerifyCommandLen {
			return nil, fmt.Errorf("%w: verify command exceeds %d characters", domain.ErrInvalidInput, domain.MaxVerifyCommandLen)
		}
		expected := strings.TrimSpace(raw.ExpectedOutcome)
		if len(expected) > domain.MaxVerifyExpectedOutcomeLen {
			return nil, fmt.Errorf("%w: expected_outcome exceeds %d characters", domain.ErrInvalidInput, domain.MaxVerifyExpectedOutcomeLen)
		}
		out = append(out, VerifyCommandInput{
			Command:         cmd,
			ExpectedOutcome: expected,
		})
	}
	if len(out) > domain.MaxVerifyCommandsPerItem {
		return nil, fmt.Errorf("%w: at most %d verify commands per criterion", domain.ErrInvalidInput, domain.MaxVerifyCommandsPerItem)
	}
	return out, nil
}

func replaceCommandsInTx(tx *gorm.DB, itemID string, cmds []VerifyCommandInput) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.checklist.replaceCommandsInTx")
	if err := tx.Where("item_id = ?", itemID).Delete(&model.TaskChecklistItemCommand{}).Error; err != nil {
		return fmt.Errorf("delete verify commands: %w", err)
	}
	for i, c := range cmds {
		drow := domain.TaskChecklistItemCommand{
			ID:              uuid.NewString(),
			ItemID:          itemID,
			SortOrder:       i,
			Command:         c.Command,
			ExpectedOutcome: c.ExpectedOutcome,
		}
		mrow := model.FromDomainTaskChecklistItemCommand(drow)
		if err := tx.Create(&mrow).Error; err != nil {
			return fmt.Errorf("insert verify command: %w", err)
		}
	}
	return nil
}

func commandsForItemsInTx(tx *gorm.DB, itemIDs []string) (map[string][]VerifyCommandView, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.checklist.commandsForItemsInTx")
	if len(itemIDs) == 0 {
		return map[string][]VerifyCommandView{}, nil
	}
	var rows []model.TaskChecklistItemCommand
	if err := tx.Where("item_id IN ?", itemIDs).Order("item_id ASC, sort_order ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list verify commands: %w", err)
	}
	out := make(map[string][]VerifyCommandView, len(itemIDs))
	for _, r := range rows {
		dr := model.ToDomainTaskChecklistItemCommand(r)
		out[dr.ItemID] = append(out[dr.ItemID], VerifyCommandView{
			SortOrder:       dr.SortOrder,
			Command:         dr.Command,
			ExpectedOutcome: dr.ExpectedOutcome,
		})
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func commandsForItemInTx(tx *gorm.DB, itemID string) ([]VerifyCommandView, error) {
	m, err := commandsForItemsInTx(tx, []string{itemID})
	if err != nil {
		return nil, err
	}
	return m[itemID], nil
}

// ReplaceVerifyCommands replaces all verify commands for an item owned by taskID.
func ReplaceVerifyCommands(ctx context.Context, db *gorm.DB, taskID, itemID string, cmds []VerifyCommandInput, by domain.Actor) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.checklist.ReplaceVerifyCommands")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	normalized, err := NormalizeVerifyCommandInputs(cmds)
	if err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	itemID = strings.TrimSpace(itemID)
	if taskID == "" || itemID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadItemForCommandEdit(tx, taskID, itemID); err != nil {
			return err
		}
		return replaceCommandsInTx(tx, itemID, normalized)
	})
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func loadItemForCommandEdit(tx *gorm.DB, taskID, itemID string) (*domain.TaskChecklistItem, error) {
	t, err := kernel.LoadTask(tx, taskID)
	if err != nil {
		return nil, err
	}
	if err := ValidateCriteriaMutable(t); err != nil {
		return nil, err
	}
	var it model.TaskChecklistItem
	if err := tx.Where("id = ? AND task_id = ?", itemID, taskID).First(&it).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("load checklist item: %w", err)
	}
	var doneCount int64
	if err := tx.Model(&model.TaskChecklistCompletion{}).
		Where("item_id = ?", itemID).
		Count(&doneCount).Error; err != nil {
		return nil, fmt.Errorf("count completions: %w", err)
	}
	if criterionLockedByCompletion(t.Status, doneCount) {
		return nil, fmt.Errorf("%w: cannot edit verify commands on a criterion that has already been marked done", domain.ErrInvalidInput)
	}
	dit := model.ToDomainTaskChecklistItem(it)
	return &dit, nil
}
