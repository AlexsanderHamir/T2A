package stats

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// scan_cycles.go owns the cycle/phase aggregation queries that back the
// `cycles` and `phases` blocks of GET /tasks/stats. The HTTP wire shape
// (always-present keys, `{}` on empty database) is pinned by
// handler_http_list_stats_contract_test.go in the handler package.

type cycleStatusCountRow struct {
	Status domain.CycleStatus
	Count  int64
}

func scanCyclesByStatus(ctx context.Context, db *gorm.DB) ([]cycleStatusCountRow, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanCyclesByStatus")
	var rows []cycleStatusCountRow
	if err := db.WithContext(ctx).Model(&domain.TaskCycle{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("cycles by status: %w", err)
	}
	return rows, nil
}

type cycleActorCountRow struct {
	TriggeredBy domain.Actor `gorm:"column:triggered_by"`
	Count       int64
}

func scanCyclesByTriggeredBy(ctx context.Context, db *gorm.DB) ([]cycleActorCountRow, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanCyclesByTriggeredBy")
	var rows []cycleActorCountRow
	if err := db.WithContext(ctx).Model(&domain.TaskCycle{}).
		Select("triggered_by, COUNT(*) AS count").
		Group("triggered_by").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("cycles by triggered_by: %w", err)
	}
	return rows, nil
}

type phaseStatusCountRow struct {
	Phase  domain.Phase
	Status domain.PhaseStatus
	Count  int64
}

// scanPhasesByStatus aggregates task_cycle_phases by (phase, status). The
// pair is the source of truth for "which phase did failures concentrate
// in" — exposed verbatim under `phases.by_phase_status[phase][status]` on
// the wire so a future column rename trips both this scanner and the
// frontend heatmap in the same PR.
func scanPhasesByStatus(ctx context.Context, db *gorm.DB) ([]phaseStatusCountRow, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanPhasesByStatus")
	var rows []phaseStatusCountRow
	if err := db.WithContext(ctx).Model(&domain.TaskCyclePhase{}).
		Select("phase, status, COUNT(*) AS count").
		Group("phase, status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("phases by status: %w", err)
	}
	return rows, nil
}
