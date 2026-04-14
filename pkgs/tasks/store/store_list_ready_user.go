package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ListReadyTasksUserCreated returns tasks with status ready whose first audit row is
// task_created by user (matches the user-task agent queue policy). Results are ordered by id ascending.
// afterID, when non-empty after trim, restricts to tasks.id > afterID for pagination.
func (s *Store) ListReadyTasksUserCreated(ctx context.Context, limit int, afterID string) ([]domain.Task, error) {
	defer deferStoreLatency(storeOpListReadyUserCreated)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListReadyTasksUserCreated")
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	afterID = strings.TrimSpace(afterID)
	q := s.db.WithContext(ctx).Model(&domain.Task{}).
		Joins(`INNER JOIN task_events te ON te.task_id = tasks.id AND te.seq = ? AND te.type = ? AND te.by = ?`,
			int64(1), domain.EventTaskCreated, domain.ActorUser).
		Where("tasks.status = ?", domain.StatusReady).
		Order("tasks.id ASC").
		Limit(limit)
	if afterID != "" {
		q = q.Where("tasks.id > ?", afterID)
	}
	var out []domain.Task
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list ready tasks user created: %w", err)
	}
	return out, nil
}
