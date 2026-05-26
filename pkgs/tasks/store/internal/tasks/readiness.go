package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// DependenciesSatisfied reports whether every task_dependencies predecessor is done.
func DependenciesSatisfied(ctx context.Context, db *gorm.DB, taskID string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.DependenciesSatisfied")
	var n int64
	err := db.WithContext(ctx).Model(&domain.TaskDependency{}).
		Joins("INNER JOIN tasks dep ON dep.id = task_dependencies.depends_on_task_id").
		Where("task_dependencies.task_id = ? AND dep.status <> ?", taskID, domain.StatusDone).
		Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("count open dependencies: %w", err)
	}
	return n == 0, nil
}

// ReadyForAgentPickup applies the same predicates as ListQueueCandidates for one task row.
func ReadyForAgentPickup(ctx context.Context, db *gorm.DB, t *domain.Task, now time.Time) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ReadyForAgentPickup")
	if t == nil || t.Status != domain.StatusReady {
		return false, nil
	}
	if t.PickupNotBefore != nil && t.PickupNotBefore.After(now) {
		return false, nil
	}
	if t.Gate != nil && t.Gate.GateBlocksWorker() {
		return false, nil
	}
	return DependenciesSatisfied(ctx, db, t.ID)
}
