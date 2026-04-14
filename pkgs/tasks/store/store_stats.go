package store

import (
	"context"
	"log/slog"

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
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TaskStats")
	r, err := s.scanTaskStatsTotals(ctx)
	if err != nil {
		return TaskStats{}, err
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
	statusRows, err := s.scanTaskStatsByStatus(ctx)
	if err != nil {
		return TaskStats{}, err
	}
	for _, sr := range statusRows {
		stats.ByStatus[sr.Status] = sr.Count
	}
	priorityRows, err := s.scanTaskStatsByPriority(ctx)
	if err != nil {
		return TaskStats{}, err
	}
	for _, pr := range priorityRows {
		stats.ByPriority[pr.Priority] = pr.Count
	}
	return stats, nil
}
