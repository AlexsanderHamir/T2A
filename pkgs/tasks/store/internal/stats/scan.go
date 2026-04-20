package stats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

type totalsRow struct {
	Total        int64
	Ready        int64
	Critical     int64
	ParentTotal  int64
	SubtaskTotal int64
	// Scheduled is the number of ready tasks deferred into the
	// future via `pickup_not_before > now`. It is the SQL-side
	// projection of the same predicate the agent's
	// `ready.ListQueueCandidates` uses to *exclude* a row from the
	// queue, surfaced as a stats counter so the Observability page
	// can answer "0 ready, 12 scheduled" (intentionally deferred)
	// vs "0 ready, 0 scheduled" (truly idle). Driven by the
	// existing index on `tasks.pickup_not_before` — no new schema.
	Scheduled int64
}

func scanTotals(ctx context.Context, db *gorm.DB, now time.Time) (totalsRow, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanTotals")
	var r totalsRow
	// We embed the StatusReady literal inline for the `scheduled`
	// CASE rather than threading another `?` through the Select
	// args. GORM's Select binds positional args left-to-right
	// across the entire compiled SQL — including WHERE clauses
	// implicitly added by Model(&domain.Task{}) (soft-delete
	// gating). The four literal `?` slots stay tied to ready /
	// critical / ready (scheduled gate) / now in source order, but
	// reusing the same enum value via interpolation removes the
	// "is the 3rd `?` actually our 3rd arg?" ambiguity that bit
	// the regression test on first authoring. domain.StatusReady
	// is a const string and not user-controlled, so there is no
	// injection surface here.
	scheduledClause := fmt.Sprintf(
		"SUM(CASE WHEN status = '%s' AND pickup_not_before IS NOT NULL AND pickup_not_before > ? THEN 1 ELSE 0 END) AS scheduled",
		string(domain.StatusReady),
	)
	err := db.WithContext(ctx).Model(&domain.Task{}).
		Select(
			"COUNT(*) AS total, "+
				"SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS ready, "+
				"SUM(CASE WHEN priority = ? THEN 1 ELSE 0 END) AS critical, "+
				"SUM(CASE WHEN parent_id IS NULL OR parent_id = '' THEN 1 ELSE 0 END) AS parent_total, "+
				"SUM(CASE WHEN parent_id IS NOT NULL AND parent_id <> '' THEN 1 ELSE 0 END) AS subtask_total, "+
				scheduledClause,
			domain.StatusReady,
			domain.PriorityCritical,
			now,
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

func scanByStatus(ctx context.Context, db *gorm.DB) ([]statusCountRow, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanByStatus")
	var statusRows []statusCountRow
	if err := db.WithContext(ctx).Model(&domain.Task{}).
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

func scanByPriority(ctx context.Context, db *gorm.DB) ([]priorityCountRow, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanByPriority")
	var priorityRows []priorityCountRow
	if err := db.WithContext(ctx).Model(&domain.Task{}).
		Select("priority, COUNT(*) AS count").
		Group("priority").
		Scan(&priorityRows).Error; err != nil {
		return nil, fmt.Errorf("task stats by priority: %w", err)
	}
	return priorityRows, nil
}
