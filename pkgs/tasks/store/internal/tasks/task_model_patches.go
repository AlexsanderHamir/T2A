package tasks

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func applyGatePatch(cur *domain.Task, gate **domain.TaskGate) error {
	if gate == nil {
		return nil
	}
	cur.Gate = *gate
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func applyDependsOnPatch(tx *gorm.DB, taskID string, cur *domain.Task, dependsOn *[]domain.DependencyEdge) error {
	if dependsOn == nil {
		return nil
	}
	if err := setDependenciesInTx(tx, taskID, *dependsOn); err != nil {
		return err
	}
	edges, err := listDependencyEdgesInTx(tx, taskID)
	if err != nil {
		return err
	}
	cur.DependsOn = edges
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func listDependencyEdgesInTx(tx *gorm.DB, taskID string) ([]domain.DependencyEdge, error) {
	var rows []domain.TaskDependency
	err := tx.Where("task_id = ?", taskID).Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
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
func listDependenciesInTx(tx *gorm.DB, taskID string) ([]string, error) {
	edges, err := listDependencyEdgesInTx(tx, taskID)
	if err != nil {
		return nil, err
	}
	return DependencyEdgeIDs(edges), nil
}

func setDependenciesInTx(tx *gorm.DB, taskID string, dependsOn []domain.DependencyEdge) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.tasks.setDependenciesInTx")
	normalized, err := normalizeDependencyEdges(taskID, dependsOn)
	if err != nil {
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
	for _, e := range normalized {
		row := domain.TaskDependency{
			TaskID:          taskID,
			DependsOnTaskID: e.TaskID,
			Satisfies:       e.Satisfies,
			CreatedAt:       time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create dependency: %w", err)
		}
	}
	return nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
