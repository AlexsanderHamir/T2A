package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentPickupResult is returned when the worker atomically transitions a task
// from ready to running and consumes any pending retry intent.
type AgentPickupResult struct {
	Task          *domain.Task
	ConsumedRetry *domain.PendingRetry
}

// AgentPickup locks the task, requires status=ready, flips to running, clears
// pending_retry, and returns a copy of the consumed intent (if any).
func AgentPickup(ctx context.Context, db *gorm.DB, taskID string, by domain.Actor) (*AgentPickupResult, error) {
	defer kernel.DeferLatency(kernel.OpUpdateTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.AgentPickup", "task_id", taskID)
	if err := kernel.ValidateActor(by); err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var out AgentPickupResult
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("load task: %w", err)
		}
		if cur.Status != domain.StatusReady {
			return fmt.Errorf("%w: task status is %q, want ready", domain.ErrInvalidInput, cur.Status)
		}
		out.ConsumedRetry = cur.PendingRetry.Clone()
		cur.PendingRetry = nil
		running := domain.StatusRunning
		nextSeq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		if err := applyStatusPatch(tx, taskID, &cur, &running, by, &nextSeq); err != nil {
			return err
		}
		if err := tx.Save(&cur).Error; err != nil {
			return fmt.Errorf("save task: %w", err)
		}
		if err := hydrateDependsOn(ctx, tx, &cur); err != nil {
			return err
		}
		out.Task = &cur
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}
