package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/scheduling"
	"gorm.io/gorm"
)

// ReadyForAgentPickup applies the same predicates as ListQueueCandidates for one task row.
func ReadyForAgentPickup(ctx context.Context, db *gorm.DB, t *domain.Task, now time.Time) (bool, scheduling.FailedPredicate, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ReadyForAgentPickup")
	partial := scheduling.EvaluateWorkerReadiness(t, now, false)
	if !partial.Ready && partial.FailedPredicate != scheduling.FailedPredicateDependencies {
		return false, partial.FailedPredicate, nil
	}
	depsMet, err := DependenciesSatisfied(ctx, db, t.ID)
	if err != nil {
		return false, scheduling.FailedPredicateDependencies, err
	}
	result := scheduling.EvaluateWorkerReadiness(t, now, depsMet)
	return result.Ready, result.FailedPredicate, nil
}
