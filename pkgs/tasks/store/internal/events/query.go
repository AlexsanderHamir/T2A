package events

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

// List returns audit events for a task in ascending sequence order.
// Used for full-history reads (small tasks) and as the canonical
// ordering reference for tests; pagination should go through PageCursor.
func List(ctx context.Context, db *gorm.DB, taskID string) ([]domain.TaskEvent, error) {
	defer kernel.DeferLatency(kernel.OpListTaskEvents)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.events.List")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskEvent
	err := db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("seq ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list task events: %w", err)
	}
	return rows, nil
}

// Count returns how many audit rows exist for taskID.
func Count(ctx context.Context, db *gorm.DB, taskID string) (int64, error) {
	defer kernel.DeferLatency(kernel.OpTaskEventCount)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.events.Count")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 0, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var n int64
	err := db.WithContext(ctx).Model(&domain.TaskEvent{}).Where("task_id = ?", taskID).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count task events: %w", err)
	}
	return n, nil
}

// LastSeq returns the highest seq for taskID, or 0 when there are no events.
// Used by the SSE backfill cursor and reconcile to skip already-shipped rows.
func LastSeq(ctx context.Context, db *gorm.DB, taskID string) (int64, error) {
	defer kernel.DeferLatency(kernel.OpLastEventSeq)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.events.LastSeq")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 0, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var maxSeq int64
	err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(MAX(seq), 0)").
		Scan(&maxSeq).Error
	if err != nil {
		return 0, fmt.Errorf("last event seq: %w", err)
	}
	return maxSeq, nil
}

// Get returns one task_events row by composite key (task_id, seq), or
// domain.ErrNotFound when the row does not exist.
func Get(ctx context.Context, db *gorm.DB, taskID string, seq int64) (*domain.TaskEvent, error) {
	defer kernel.DeferLatency(kernel.OpGetTaskEvent)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.events.Get")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if seq < 1 {
		return nil, fmt.Errorf("%w: seq", domain.ErrInvalidInput)
	}
	var ev domain.TaskEvent
	err := db.WithContext(ctx).Where("task_id = ? AND seq = ?", taskID, seq).First(&ev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get task event: %w", err)
	}
	return &ev, nil
}
