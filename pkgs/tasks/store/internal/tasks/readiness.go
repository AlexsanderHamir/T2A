package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// DependenciesSatisfied reports whether every task_dependencies predecessor
// meets its edge predicate (done or criteria_complete).
func DependenciesSatisfied(ctx context.Context, db *gorm.DB, taskID string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.DependenciesSatisfied")
	edges, err := ListDependencyEdges(ctx, db, taskID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		var dep domain.Task
		if err := db.WithContext(ctx).Where("id = ?", e.TaskID).First(&dep).Error; err != nil {
			return false, fmt.Errorf("load dependency predecessor: %w", err)
		}
		if !EdgeSatisfied(&dep, e.Satisfies) {
			return false, nil
		}
	}
	return true, nil
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
	awaiting, err := ParentAwaitingSubtasks(ctx, db, t)
	if err != nil {
		return false, err
	}
	if awaiting {
		return false, nil
	}
	return DependenciesSatisfied(ctx, db, t.ID)
}
