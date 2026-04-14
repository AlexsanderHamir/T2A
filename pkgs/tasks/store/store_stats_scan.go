package store

import (
	"context"
	"fmt"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type taskStatsTotalsRow struct {
	Total        int64
	Ready        int64
	Critical     int64
	ParentTotal  int64
	SubtaskTotal int64
}

func (s *Store) scanTaskStatsTotals(ctx context.Context) (taskStatsTotalsRow, error) {
	var r taskStatsTotalsRow
	err := s.db.WithContext(ctx).Model(&domain.Task{}).
		Select(
			"COUNT(*) AS total, "+
				"SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS ready, "+
				"SUM(CASE WHEN priority = ? THEN 1 ELSE 0 END) AS critical, "+
				"SUM(CASE WHEN parent_id IS NULL OR parent_id = '' THEN 1 ELSE 0 END) AS parent_total, "+
				"SUM(CASE WHEN parent_id IS NOT NULL AND parent_id <> '' THEN 1 ELSE 0 END) AS subtask_total",
			domain.StatusReady,
			domain.PriorityCritical,
		).
		Scan(&r).Error
	if err != nil {
		return r, fmt.Errorf("task stats: %w", err)
	}
	return r, nil
}

type statusCountRow struct {
	Status domain.Status
	Count  int64
}

func (s *Store) scanTaskStatsByStatus(ctx context.Context) ([]statusCountRow, error) {
	var statusRows []statusCountRow
	if err := s.db.WithContext(ctx).Model(&domain.Task{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&statusRows).Error; err != nil {
		return nil, fmt.Errorf("task stats by status: %w", err)
	}
	return statusRows, nil
}

type priorityCountRow struct {
	Priority domain.Priority
	Count    int64
}

func (s *Store) scanTaskStatsByPriority(ctx context.Context) ([]priorityCountRow, error) {
	var priorityRows []priorityCountRow
	if err := s.db.WithContext(ctx).Model(&domain.Task{}).
		Select("priority, COUNT(*) AS count").
		Group("priority").
		Scan(&priorityRows).Error; err != nil {
		return nil, fmt.Errorf("task stats by priority: %w", err)
	}
	return priorityRows, nil
}
