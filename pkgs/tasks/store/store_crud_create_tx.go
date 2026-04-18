package store

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/drafts"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

func createTaskInTx(tx *gorm.DB, t *domain.Task, in CreateTaskInput, by domain.Actor, title string, parentID *string, st domain.Status) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.createTaskInTx")
	if parentID != nil {
		var n int64
		if err := tx.Model(&domain.Task{}).Where("id = ?", *parentID).Count(&n).Error; err != nil {
			return fmt.Errorf("parent lookup: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: parent not found", domain.ErrInvalidInput)
		}
	}
	if err := tx.Create(t).Error; err != nil {
		if isDuplicateTaskPrimaryKey(err) {
			return fmt.Errorf("%w: task id already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert task: %w", err)
	}
	seq := int64(1)
	if err := attachDraftEvaluationsTx(tx, in.DraftID, t.ID); err != nil {
		return err
	}
	if err := drafts.DeleteByIDInTx(tx, in.DraftID); err != nil {
		return err
	}
	if err := kernel.AppendEvent(tx, t.ID, seq, domain.EventTaskCreated, by, nil); err != nil {
		return err
	}
	seq++
	if parentID != nil {
		pseq, err := kernel.NextEventSeq(tx, *parentID)
		if err != nil {
			return err
		}
		pb, err := json.Marshal(map[string]string{
			"child_task_id": t.ID,
			"title":         title,
		})
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, *parentID, pseq, domain.EventSubtaskAdded, by, pb); err != nil {
			return err
		}
	}
	if st == domain.StatusDone {
		if err := validateCanMarkDoneTx(tx, t.ID); err != nil {
			return err
		}
	}
	return nil
}
