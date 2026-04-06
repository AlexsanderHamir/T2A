package store

import (
	"context"
	"fmt"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type TaskStats struct {
	Total      int64
	Ready      int64
	Critical   int64
	ByStatus   map[domain.Status]int64
	ByPriority map[domain.Priority]int64
	ByScope    map[string]int64
}

// TaskStats returns global counters across all tasks.
func (s *Store) TaskStats(ctx context.Context) (TaskStats, error) {
	type row struct {
		Total        int64
		Ready        int64
		Critical     int64
		ParentTotal  int64
		SubtaskTotal int64
	}
	var r row
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
		return TaskStats{}, fmt.Errorf("task stats: %w", err)
	}
	stats := TaskStats{
		Total:      r.Total,
		Ready:      r.Ready,
		Critical:   r.Critical,
		ByStatus:   map[domain.Status]int64{},
		ByPriority: map[domain.Priority]int64{},
		ByScope: map[string]int64{
			"parent":  r.ParentTotal,
			"subtask": r.SubtaskTotal,
		},
	}

	type statusCountRow struct {
		Status domain.Status
		Count  int64
	}
	var statusRows []statusCountRow
	if err := s.db.WithContext(ctx).Model(&domain.Task{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&statusRows).Error; err != nil {
		return TaskStats{}, fmt.Errorf("task stats by status: %w", err)
	}
	for _, sr := range statusRows {
		stats.ByStatus[sr.Status] = sr.Count
	}

	type priorityCountRow struct {
		Priority domain.Priority
		Count    int64
	}
	var priorityRows []priorityCountRow
	if err := s.db.WithContext(ctx).Model(&domain.Task{}).
		Select("priority, COUNT(*) AS count").
		Group("priority").
		Scan(&priorityRows).Error; err != nil {
		return TaskStats{}, fmt.Errorf("task stats by priority: %w", err)
	}
	for _, pr := range priorityRows {
		stats.ByPriority[pr.Priority] = pr.Count
	}

	return stats, nil
}
