package cycles

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// ListRunning returns every task_cycles row with status='running'
// across all tasks. Ordered by created_at ASC for deterministic
// startup-sweep behaviour. Used by the agent worker's startup orphan
// sweep (docs/AGENT-WORKER-PLAN.md Stage 4); unbounded by design — V1
// expects very few orphan rows and an explicit cap would silently drop
// rows that need cleanup.
func ListRunning(ctx context.Context, db *gorm.DB) ([]domain.TaskCycle, error) {
	defer kernel.DeferLatency(kernel.OpListCyclesForTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.ListRunning")
	var out []domain.TaskCycle
	q := db.WithContext(ctx).
		Where("status = ?", domain.CycleStatusRunning).
		Order("started_at ASC, id ASC")
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list running task_cycles: %w", err)
	}
	return out, nil
}

// ListRunningPhases returns every task_cycle_phases row with
// status='running' across all cycles. Ordered by started_at ASC, id
// ASC. Used by the startup orphan sweep to clean up phase rows whose
// parent cycle has already been closed (race where the cycle was
// terminated while a phase write was in flight).
func ListRunningPhases(ctx context.Context, db *gorm.DB) ([]domain.TaskCyclePhase, error) {
	defer kernel.DeferLatency(kernel.OpListCyclePhases)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.ListRunningPhases")
	var out []domain.TaskCyclePhase
	q := db.WithContext(ctx).
		Where("status = ?", domain.PhaseStatusRunning).
		Order("started_at ASC, id ASC")
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list running task_cycle_phases: %w", err)
	}
	return out, nil
}
