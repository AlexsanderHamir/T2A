// Package stats owns the global task-counters query that backs GET
// /tasks/stats. The public store facade re-exports TaskStats and the
// Get function via (*Store).TaskStats. The shape (Total / Ready /
// Critical / ByStatus / ByPriority / ByScope) is the HTTP response
// contract — see handler_http_list_stats_contract_test.go.
package stats

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

// TaskStats holds the global task counters. Tests pin the invariant
// that every domain.Status / domain.Priority appears in the maps so
// downstream consumers do not need to nil-check; Get materializes the
// maps eagerly.
type TaskStats struct {
	Total      int64
	Ready      int64
	Critical   int64
	ByStatus   map[domain.Status]int64
	ByPriority map[domain.Priority]int64
	ByScope    map[string]int64
}

// Get returns global counters across all tasks. Three SQL round-trips:
// totals, group-by-status, group-by-priority. ByScope ("parent" /
// "subtask") is derived from the totals row.
func Get(ctx context.Context, db *gorm.DB) (TaskStats, error) {
	defer kernel.DeferLatency(kernel.OpTaskStats)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.Get")
	r, err := scanTotals(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	out := TaskStats{
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
	statusRows, err := scanByStatus(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, sr := range statusRows {
		out.ByStatus[sr.Status] = sr.Count
	}
	priorityRows, err := scanByPriority(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, pr := range priorityRows {
		out.ByPriority[pr.Priority] = pr.Count
	}
	return out, nil
}
