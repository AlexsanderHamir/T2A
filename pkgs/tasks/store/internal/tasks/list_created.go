package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/gorm"
)

const (
	taskCreatedEventSeq  = int64(1)
	listOrderCreatedDesc = "te.at DESC, tasks.id DESC"
)

type listRowScan struct {
	domain.Task
	TaskCreatedAt time.Time `gorm:"column:task_created_at"`
}

//funclogmeasure:skip category=hot-path reason="Pure query builder without I/O; operation trace is emitted by the calling chokepoint."
func applyTaskCreatedJoin(q *gorm.DB) *gorm.DB {
	return q.
		Select("tasks.*, te.at AS task_created_at").
		Joins(`INNER JOIN task_events te ON te.task_id = tasks.id AND te.seq = ? AND te.type = ?`,
			taskCreatedEventSeq, domain.EventTaskCreated)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func tasksFromListRows(rows []listRowScan) []domain.Task {
	out := make([]domain.Task, len(rows))
	for i, r := range rows {
		t := r.Task
		if !r.TaskCreatedAt.IsZero() {
			at := r.TaskCreatedAt.UTC()
			t.CreatedAt = &at
		}
		out[i] = t
	}
	return out
}

func hydrateCreatedAt(ctx context.Context, db *gorm.DB, t *domain.Task) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.hydrateCreatedAt")
	if t == nil || strings.TrimSpace(t.ID) == "" {
		return nil
	}
	var at time.Time
	err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("task_id = ? AND seq = ? AND type = ?", t.ID, taskCreatedEventSeq, domain.EventTaskCreated).
		Select("at").
		Scan(&at).Error
	if err != nil {
		return fmt.Errorf("load task created_at: %w", err)
	}
	if !at.IsZero() {
		utc := at.UTC()
		t.CreatedAt = &utc
	}
	return nil
}

func loadCreatedAtCursor(ctx context.Context, db *gorm.DB, afterID string) (time.Time, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.loadCreatedAtCursor")
	var at time.Time
	err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Where("task_id = ? AND seq = ? AND type = ?", afterID, taskCreatedEventSeq, domain.EventTaskCreated).
		Select("at").
		Scan(&at).Error
	if err != nil {
		return time.Time{}, fmt.Errorf("load list cursor: %w", err)
	}
	if at.IsZero() {
		return time.Time{}, domain.ErrNotFound
	}
	return at.UTC(), nil
}
