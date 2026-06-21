package tasks

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/scheduling"
	"gorm.io/gorm"
)

// DependenciesSatisfied reports whether every predecessor edge for taskID is met.
func DependenciesSatisfied(ctx context.Context, db *gorm.DB, taskID string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.DependenciesSatisfied")
	edges, err := ListDependencyEdges(ctx, db, taskID)
	if err != nil {
		return false, err
	}
	for _, e := range edges {
		predecessor, err := Get(ctx, db, e.TaskID)
		if err != nil {
			return false, err
		}
		if !scheduling.EdgeSatisfied(predecessor, e.Satisfies) {
			return false, nil
		}
	}
	return true, nil
}
