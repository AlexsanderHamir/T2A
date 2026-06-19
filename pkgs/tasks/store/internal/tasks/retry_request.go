package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RequestRetryInput is the store payload for operator retry after failure.
type RequestRetryInput struct {
	TaskID        string
	Mode          domain.RetryMode
	ParentCycleID string // optional; latest terminal cycle when empty
}

// RequestTaskRetry sets pending_retry and status=ready for a failed task.
// Returns (task, prevStatus, err). Idempotent when the task is already ready
// with the same pending_retry payload.
func RequestTaskRetry(ctx context.Context, db *gorm.DB, in RequestRetryInput, by domain.Actor) (*domain.Task, domain.Status, error) {
	defer kernel.DeferLatency(kernel.OpUpdateTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.RequestTaskRetry", "task_id", in.TaskID)
	if err := kernel.ValidateActor(by); err != nil {
		return nil, "", err
	}
	taskID := strings.TrimSpace(in.TaskID)
	if taskID == "" {
		return nil, "", fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	intent := domain.PendingRetry{
		Mode:          in.Mode,
		ParentCycleID: strings.TrimSpace(in.ParentCycleID),
	}
	var updated *domain.Task
	var origStatus domain.Status
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		origStatus = cur.Status
		parentID, err := resolveRetryParentCycleInTx(tx, taskID, intent.ParentCycleID)
		if err != nil {
			return err
		}
		intent.ParentCycleID = parentID
		if err := intent.Validate(); err != nil {
			return err
		}
		if cur.Status == domain.StatusReady && cur.PendingRetry != nil {
			if cur.PendingRetry.Equal(&intent) {
				updated = &cur
				return nil
			}
			return fmt.Errorf("%w: task already queued with different retry intent", domain.ErrConflict)
		}
		if cur.Status != domain.StatusFailed {
			return fmt.Errorf("%w: task status is %q, want failed", domain.ErrInvalidInput, cur.Status)
		}
		nextSeq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]string{
			"mode":            string(intent.Mode),
			"parent_cycle_id": intent.ParentCycleID,
		})
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, taskID, nextSeq, domain.EventTaskRetryRequested, by, payload); err != nil {
			return err
		}
		nextSeq++
		cur.PendingRetry = &intent
		ready := domain.StatusReady
		if err := applyStatusPatch(tx, taskID, &cur, &ready, by, &nextSeq); err != nil {
			return err
		}
		if err := tx.Save(&cur).Error; err != nil {
			return fmt.Errorf("save task: %w", err)
		}
		if err := hydrateDependsOn(ctx, tx, &cur); err != nil {
			return err
		}
		updated = &cur
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, "", domain.ErrNotFound
		}
		return nil, "", err
	}
	return updated, origStatus, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func resolveRetryParentCycleInTx(tx *gorm.DB, taskID, explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		var c domain.TaskCycle
		if err := tx.Where("id = ?", explicit).First(&c).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", domain.ErrNotFound
			}
			return "", fmt.Errorf("load parent cycle: %w", err)
		}
		if c.TaskID != taskID {
			return "", fmt.Errorf("%w: parent_cycle_id does not belong to this task", domain.ErrInvalidInput)
		}
		if !domain.TerminalCycleStatus(c.Status) {
			return "", fmt.Errorf("%w: parent cycle is not terminal", domain.ErrInvalidInput)
		}
		return c.ID, nil
	}
	var cycles []domain.TaskCycle
	if err := tx.Where("task_id = ?", taskID).Order("attempt_seq DESC").Limit(50).Find(&cycles).Error; err != nil {
		return "", fmt.Errorf("list cycles: %w", err)
	}
	for i := range cycles {
		if domain.TerminalCycleStatus(cycles[i].Status) {
			return cycles[i].ID, nil
		}
	}
	return "", fmt.Errorf("%w: no terminal cycle to retry from", domain.ErrInvalidInput)
}
