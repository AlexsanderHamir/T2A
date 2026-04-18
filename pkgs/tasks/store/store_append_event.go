package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// AppendTaskEvent appends one task_events row if the task exists.
func (s *Store) AppendTaskEvent(ctx context.Context, taskID string, typ domain.EventType, by domain.Actor, data []byte) error {
	defer kernel.DeferLatency(kernel.OpAppendTaskEvent)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AppendTaskEvent")
	if err := kernel.ValidateActor(by); err != nil {
		return err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&domain.Task{}).Where("id = ?", taskID).Count(&n).Error; err != nil {
			return fmt.Errorf("task lookup: %w", err)
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		seq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		return kernel.AppendEvent(tx, taskID, seq, typ, by, data)
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("append task event: %w", err)
	}
	return nil
}
