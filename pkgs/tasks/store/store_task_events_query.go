package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
)

// ListTaskEvents returns audit events for a task in ascending sequence order.
func (s *Store) ListTaskEvents(ctx context.Context, taskID string) ([]domain.TaskEvent, error) {
	defer kernel.DeferLatency(kernel.OpListTaskEvents)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListTaskEvents")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var events []domain.TaskEvent
	err := s.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("seq ASC").
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("list task events: %w", err)
	}
	return events, nil
}

// TaskEventCount returns how many audit rows exist for the task.
func (s *Store) TaskEventCount(ctx context.Context, taskID string) (int64, error) {
	defer kernel.DeferLatency(kernel.OpTaskEventCount)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TaskEventCount")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 0, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var n int64
	err := s.db.WithContext(ctx).Model(&domain.TaskEvent{}).Where("task_id = ?", taskID).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count task events: %w", err)
	}
	return n, nil
}

// LastEventSeq returns the highest seq for the task, or 0 when there are no events.
func (s *Store) LastEventSeq(ctx context.Context, taskID string) (int64, error) {
	defer kernel.DeferLatency(kernel.OpLastEventSeq)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.LastEventSeq")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 0, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var maxSeq int64
	err := s.db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(MAX(seq), 0)").
		Scan(&maxSeq).Error
	if err != nil {
		return 0, fmt.Errorf("last event seq: %w", err)
	}
	return maxSeq, nil
}
