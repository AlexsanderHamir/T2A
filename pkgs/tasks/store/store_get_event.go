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

// GetTaskEvent returns one task_events row by composite key, or ErrNotFound.
func (s *Store) GetTaskEvent(ctx context.Context, taskID string, seq int64) (*domain.TaskEvent, error) {
	defer kernel.DeferLatency(kernel.OpGetTaskEvent)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetTaskEvent")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if seq < 1 {
		return nil, fmt.Errorf("%w: seq", domain.ErrInvalidInput)
	}
	var ev domain.TaskEvent
	err := s.db.WithContext(ctx).Where("task_id = ? AND seq = ?", taskID, seq).First(&ev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get task event: %w", err)
	}
	return &ev, nil
}
