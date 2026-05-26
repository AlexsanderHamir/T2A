package tasks

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func applyTagsPatch(cur *domain.Task, tags *[]string) error {
	if tags == nil {
		return nil
	}
	normalized := domain.NormalizeTaskTags(*tags)
	if err := domain.ValidateTaskTags(normalized); err != nil {
		return err
	}
	cur.Tags = normalized
	return nil
}

func applyMilestonePatch(cur *domain.Task, milestone *string) error {
	if milestone == nil {
		return nil
	}
	m := strings.TrimSpace(*milestone)
	if m == "" {
		cur.Milestone = nil
		return nil
	}
	if err := domain.ValidateTaskMilestone(m); err != nil {
		return err
	}
	cur.Milestone = &m
	return nil
}

func applyGatePatch(cur *domain.Task, gate **domain.TaskGate) error {
	if gate == nil {
		return nil
	}
	cur.Gate = *gate
	return nil
}

func applyDependsOnPatch(tx *gorm.DB, taskID string, cur *domain.Task, dependsOn *[]string) error {
	if dependsOn == nil {
		return nil
	}
	if err := setDependenciesInTx(tx, taskID, *dependsOn); err != nil {
		return err
	}
	ids, err := listDependenciesInTx(tx, taskID)
	if err != nil {
		return err
	}
	cur.DependsOn = ids
	return nil
}

func listDependenciesInTx(tx *gorm.DB, taskID string) ([]string, error) {
	var ids []string
	err := tx.Model(&domain.TaskDependency{}).Where("task_id = ?", taskID).Order("created_at ASC").Pluck("depends_on_task_id", &ids).Error
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

func setDependenciesInTx(tx *gorm.DB, taskID string, dependsOn []string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.setDependenciesInTx")
	normalized := make([]string, 0, len(dependsOn))
	seen := make(map[string]struct{})
	for _, id := range dependsOn {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if id == taskID {
			return fmt.Errorf("%w: task cannot depend on itself", domain.ErrInvalidInput)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	for _, depID := range normalized {
		if err := ensureTaskExists(tx, depID); err != nil {
			return err
		}
		cycle, err := wouldCreateDependencyCycle(tx, taskID, depID)
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
	for _, depID := range normalized {
		row := domain.TaskDependency{
			TaskID:          taskID,
			DependsOnTaskID: depID,
			CreatedAt:       time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create dependency: %w", err)
		}
	}
	return nil
}

func normalizeCreateTaskModelFields(t *domain.Task, in CreateInput) error {
	tags := domain.NormalizeTaskTags(in.Tags)
	if err := domain.ValidateTaskTags(tags); err != nil {
		return err
	}
	t.Tags = tags
	if in.Milestone != nil {
		m := strings.TrimSpace(*in.Milestone)
		if m == "" {
			t.Milestone = nil
		} else {
			if err := domain.ValidateTaskMilestone(m); err != nil {
				return err
			}
			t.Milestone = &m
		}
	}
	t.Gate = in.Gate
	return nil
}
