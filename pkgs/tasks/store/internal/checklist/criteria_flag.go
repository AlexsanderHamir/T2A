package checklist

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// CriteriaFlagChange reports whether criteria_satisfied_at transitioned in a
// checklist completion write.
type CriteriaFlagChange struct {
	BecameComplete   bool
	BecameIncomplete bool
}

// IsChecklistCompleteInTx reports whether every inherited checklist item
// for subjectTaskID has a verified completion row.
func IsChecklistCompleteInTx(tx *gorm.DB, subjectTaskID string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.IsChecklistCompleteInTx")
	err := validateChecklistCompleteInTx(tx, subjectTaskID)
	if err == nil {
		return true, nil
	}
	return false, nil
}

// syncCriteriaSatisfiedAtInTx updates tasks.criteria_satisfied_at when
// checklist completeness transitions. Called inside checklist completion TX.
func syncCriteriaSatisfiedAtInTx(tx *gorm.DB, subjectTaskID string) (CriteriaFlagChange, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.syncCriteriaSatisfiedAtInTx")
	var change CriteriaFlagChange
	var t domain.Task
	if err := tx.Where("id = ?", subjectTaskID).First(&t).Error; err != nil {
		return change, fmt.Errorf("load task for criteria flag: %w", err)
	}
	wasComplete := t.CriteriaSatisfiedAt != nil
	nowComplete, err := IsChecklistCompleteInTx(tx, subjectTaskID)
	if err != nil {
		return change, err
	}
	switch {
	case nowComplete && !wasComplete:
		now := time.Now().UTC()
		if err := tx.Model(&domain.Task{}).Where("id = ?", subjectTaskID).
			Update("criteria_satisfied_at", now).Error; err != nil {
			return change, fmt.Errorf("set criteria_satisfied_at: %w", err)
		}
		change.BecameComplete = true
	case !nowComplete && wasComplete:
		if err := tx.Model(&domain.Task{}).Where("id = ?", subjectTaskID).
			Update("criteria_satisfied_at", nil).Error; err != nil {
			return change, fmt.Errorf("clear criteria_satisfied_at: %w", err)
		}
		change.BecameIncomplete = true
	}
	return change, nil
}

// validateDescendantsDoneInTx requires every direct child subtask to be done
// before a parent may reach status=done.
func validateDescendantsDoneInTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.validateDescendantsDoneInTx")
	var children []domain.Task
	if err := tx.Where("parent_id = ?", taskID).Find(&children).Error; err != nil {
		return fmt.Errorf("list subtasks: %w", err)
	}
	for _, ch := range children {
		if ch.Status != domain.StatusDone {
			return fmt.Errorf("%w: all subtasks must be done before marking this task done", domain.ErrInvalidInput)
		}
	}
	return nil
}

// BackfillCriteriaSatisfiedAt sets criteria_satisfied_at for tasks whose
// checklist is already complete. Idempotent migration helper.
func BackfillCriteriaSatisfiedAt(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.checklist.BackfillCriteriaSatisfiedAt")
	var ids []string
	if err := db.WithContext(ctx).Model(&domain.Task{}).Where("criteria_satisfied_at IS NULL").Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("list tasks for criteria backfill: %w", err)
	}
	for _, id := range ids {
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, err := syncCriteriaSatisfiedAtInTx(tx, id)
			return err
		}); err != nil {
			return fmt.Errorf("backfill criteria_satisfied_at for %s: %w", id, err)
		}
	}
	return nil
}
