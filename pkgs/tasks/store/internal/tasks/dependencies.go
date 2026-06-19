package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

// AddDependency records that taskID cannot run until dependsOnTaskID satisfies satisfies.
func AddDependency(ctx context.Context, db *gorm.DB, taskID, dependsOnTaskID string, satisfies domain.DependencySatisfies) error {
	defer kernel.DeferLatency(kernel.OpUpdateTask)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.AddDependency")
	taskID = strings.TrimSpace(taskID)
	dependsOnTaskID = strings.TrimSpace(dependsOnTaskID)
	if taskID == "" || dependsOnTaskID == "" {
		return fmt.Errorf("%w: task id and depends_on_task_id required", domain.ErrInvalidInput)
	}
	if taskID == dependsOnTaskID {
		return fmt.Errorf("%w: task cannot depend on itself", domain.ErrInvalidInput)
	}
	satisfies = domain.NormalizeDependencySatisfies(satisfies)
	if !domain.ValidDependencySatisfies(satisfies) {
		return fmt.Errorf("%w: invalid dependency satisfies", domain.ErrInvalidInput)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureTaskExists(tx, taskID); err != nil {
			return err
		}
		if err := ensureTaskExists(tx, dependsOnTaskID); err != nil {
			return err
		}
		cycle, err := wouldCreateDependencyCycle(tx, taskID, dependsOnTaskID)
		if err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("%w: dependency would create a cycle", domain.ErrInvalidInput)
		}
		row := domain.TaskDependency{
			TaskID:          taskID,
			DependsOnTaskID: dependsOnTaskID,
			Satisfies:       satisfies,
			CreatedAt:       time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			if isUniqueViolation(err) {
				return nil
			}
			return fmt.Errorf("create dependency: %w", err)
		}
		return nil
	})
}

// RemoveDependency deletes one dependency edge.
func RemoveDependency(ctx context.Context, db *gorm.DB, taskID, dependsOnTaskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.RemoveDependency")
	taskID = strings.TrimSpace(taskID)
	dependsOnTaskID = strings.TrimSpace(dependsOnTaskID)
	if taskID == "" || dependsOnTaskID == "" {
		return fmt.Errorf("%w: task id and depends_on_task_id required", domain.ErrInvalidInput)
	}
	res := db.WithContext(ctx).
		Where("task_id = ? AND depends_on_task_id = ?", taskID, dependsOnTaskID).
		Delete(&domain.TaskDependency{})
	if res.Error != nil {
		return fmt.Errorf("delete dependency: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListDependencyEdges returns predecessor edges for taskID.
func ListDependencyEdges(ctx context.Context, db *gorm.DB, taskID string) ([]domain.DependencyEdge, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListDependencyEdges")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var rows []domain.TaskDependency
	err := db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list dependencies: %w", err)
	}
	out := make([]domain.DependencyEdge, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.DependencyEdge{
			TaskID:    r.DependsOnTaskID,
			Satisfies: domain.NormalizeDependencySatisfies(r.Satisfies),
		})
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ListDependencies returns predecessor task ids for taskID.
func ListDependencies(ctx context.Context, db *gorm.DB, taskID string) ([]string, error) {
	edges, err := ListDependencyEdges(ctx, db, taskID)
	if err != nil {
		return nil, err
	}
	return DependencyEdgeIDs(edges), nil
}

// ListDependents returns task ids that depend on dependsOnTaskID.
func ListDependents(ctx context.Context, db *gorm.DB, dependsOnTaskID string) ([]string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.ListDependents")
	dependsOnTaskID = strings.TrimSpace(dependsOnTaskID)
	if dependsOnTaskID == "" {
		return nil, fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	var ids []string
	err := db.WithContext(ctx).Model(&domain.TaskDependency{}).
		Where("depends_on_task_id = ?", dependsOnTaskID).
		Order("created_at ASC").
		Pluck("task_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list dependents: %w", err)
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// SetDependencies replaces the full depends_on set for taskID.
func SetDependencies(ctx context.Context, db *gorm.DB, taskID string, dependsOn []domain.DependencyEdge) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.SetDependencies")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	normalized, err := normalizeDependencyEdges(taskID, dependsOn)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureTaskExists(tx, taskID); err != nil {
			return err
		}
		for _, e := range normalized {
			if err := ensureTaskExists(tx, e.TaskID); err != nil {
				return err
			}
			cycle, err := wouldCreateDependencyCycle(tx, taskID, e.TaskID)
			if err != nil {
				return err
			}
			if cycle {
				return fmt.Errorf("%w: dependency would create a cycle", domain.ErrInvalidInput)
			}
		}
		if err := tx.Where("task_id = ?", taskID).Delete(&domain.TaskDependency{}).Error; err != nil {
			return fmt.Errorf("clear dependencies: %w", err)
		}
		now := time.Now().UTC()
		for _, e := range normalized {
			row := domain.TaskDependency{
				TaskID:          taskID,
				DependsOnTaskID: e.TaskID,
				Satisfies:       e.Satisfies,
				CreatedAt:       now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("create dependency: %w", err)
			}
		}
		return nil
	})
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func hydrateDependsOn(ctx context.Context, db *gorm.DB, t *domain.Task) error {
	edges, err := ListDependencyEdges(ctx, db, t.ID)
	if err != nil {
		return err
	}
	t.DependsOn = edges
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func ensureTaskExists(tx *gorm.DB, id string) error {
	var n int64
	if err := tx.Model(&domain.Task{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return fmt.Errorf("task lookup: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func wouldCreateDependencyCycle(tx *gorm.DB, taskID, dependsOnTaskID string) (bool, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.wouldCreateDependencyCycle")
	if taskID == dependsOnTaskID {
		return true, nil
	}
	queue := []string{dependsOnTaskID}
	seen := map[string]bool{dependsOnTaskID: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		var next []string
		if err := tx.Model(&domain.TaskDependency{}).Where("task_id = ?", cur).Pluck("depends_on_task_id", &next).Error; err != nil {
			return false, fmt.Errorf("walk dependencies: %w", err)
		}
		for _, id := range next {
			if id == taskID {
				return true, nil
			}
			if !seen[id] {
				seen[id] = true
				queue = append(queue, id)
			}
		}
	}
	return false, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
