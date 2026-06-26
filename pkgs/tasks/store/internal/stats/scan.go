package stats

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/model"
	"gorm.io/gorm"
)

type totalsRow struct {
	Total     int64
	Ready     int64
	Critical  int64
	Scheduled int64
}

func scanTotals(ctx context.Context, db *gorm.DB, now time.Time) (totalsRow, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.stats.scanTotals")
	var r totalsRow
	// We embed the StatusReady literal inline for the `scheduled`
	// CASE rather than threading another `?` through the Select
	// args. GORM's Select binds positional args left-to-right
	// across the entire compiled SQL — including WHERE clauses
	// implicitly added by Model(&model.Task{}) (soft-delete
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
	err := db.WithContext(ctx).Model(&model.Task{}).
		Select(
			"COUNT(*) AS total, "+
				"SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS ready, "+
				"SUM(CASE WHEN priority = ? THEN 1 ELSE 0 END) AS critical, "+
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.stats.scanByStatus")
	var statusRows []statusCountRow
	if err := db.WithContext(ctx).Model(&model.Task{}).
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
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "tasks.store.stats.scanByPriority")
	var priorityRows []priorityCountRow
	if err := db.WithContext(ctx).Model(&model.Task{}).
		Select("priority, COUNT(*) AS count").
		Group("priority").
		Scan(&priorityRows).Error; err != nil {
		return nil, fmt.Errorf("task stats by priority: %w", err)
	}
	return priorityRows, nil
}
