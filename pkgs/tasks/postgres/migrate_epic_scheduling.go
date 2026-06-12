package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func backfillDependencySatisfies(ctx context.Context, db *gorm.DB) error {
	slog.Debug("trace", "operation", "postgres.backfillDependencySatisfies")
	res := db.WithContext(ctx).Exec(`
UPDATE task_dependencies
   SET satisfies = ?
 WHERE (satisfies IS NULL OR satisfies = '' OR satisfies = ?)
   AND EXISTS (
     SELECT 1 FROM tasks t
      WHERE t.id = task_dependencies.task_id
        AND t.parent_id = task_dependencies.depends_on_task_id
   )`, string(domain.DependencySatisfiesCriteriaComplete), string(domain.DependencySatisfiesDone))
	if res.Error != nil {
		return fmt.Errorf("backfill parent dependency satisfies: %w", res.Error)
	}
	return nil
}
