package projects

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxProjectGoalCriteria = maxProjectStepCriteria

// CreateProjectGoalInput is the store input for creating a project goal.
type CreateProjectGoalInput struct {
	ID               string
	Title            string
	Description      string
	DependsOnGoalIDs []string
	Criteria         []domain.ProjectGoalCriterion
}

// UpdateProjectGoalInput is a partial update for one goal row plus gate actions.
type UpdateProjectGoalInput struct {
	Title            *string
	Description      *string
	DependsOnGoalIDs *[]string
	GateAction       *string
	Criteria         *[]domain.ProjectGoalCriterion
}

func normalizeDependsOnGoalIDs(raw []string, selfID string) ([]string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.normalizeDependsOnGoalIDs")
	if len(raw) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if id == selfID {
			return nil, fmt.Errorf("%w: goal cannot depend on itself", domain.ErrInvalidInput)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func normalizeProjectGoalCriteria(raw []domain.ProjectGoalCriterion) ([]domain.ProjectGoalCriterion, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.normalizeProjectGoalCriteria")
	if len(raw) == 0 {
		return []domain.ProjectGoalCriterion{}, nil
	}
	if len(raw) > maxProjectGoalCriteria {
		return nil, fmt.Errorf("%w: at most %d criteria per goal", domain.ErrInvalidInput, maxProjectGoalCriteria)
	}
	step := make([]domain.ProjectStepCriterion, len(raw))
	for i, c := range raw {
		step[i] = domain.ProjectStepCriterion(c)
	}
	norm, err := normalizeProjectStepCriteria(step)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProjectGoalCriterion, len(norm))
	for i := range norm {
		out[i] = domain.ProjectGoalCriterion(norm[i])
	}
	return out, nil
}

func goalDependencyGraphHasCycle(goals []domain.ProjectGoal) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.goalDependencyGraphHasCycle")
	nodeSet := make(map[string]struct{})
	for _, g := range goals {
		nodeSet[g.ID] = struct{}{}
	}
	edges := make(map[string][]string)
	for _, g := range goals {
		edges[g.ID] = append([]string(nil), g.DependsOnGoalIDs...)
	}
	state := make(map[string]uint8)
	var dfs func(string) bool
	dfs = func(u string) bool {
		switch state[u] {
		case 1:
			return true
		case 2:
			return false
		}
		state[u] = 1
		for _, v := range edges[u] {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := nodeSet[v]; !ok {
				continue
			}
			if dfs(v) {
				return true
			}
		}
		state[u] = 2
		return false
	}
	for id := range nodeSet {
		if state[id] == 0 && dfs(id) {
			return true
		}
	}
	return false
}

func allPrerequisiteGoalsReleased(status map[string]domain.ProjectStepGateStatus, deps []string) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.allPrerequisiteGoalsReleased")
	for _, id := range deps {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		s, ok := status[id]
		if !ok || s != domain.ProjectStepGateReleased {
			return false
		}
	}
	return true
}

// recalcLockedGoals promotes locked goals whose prerequisites are all released.
func recalcLockedGoals(db *gorm.DB, projectID string, now time.Time) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.recalcLockedGoals")
	var goals []domain.ProjectGoal
	if err := db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&goals).Error; err != nil {
		return err
	}
	status := make(map[string]domain.ProjectStepGateStatus, len(goals))
	for _, g := range goals {
		status[g.ID] = g.GateStatus
	}
	guard := len(goals) + 2
	for iter := 0; iter < guard; iter++ {
		changed := false
		for i := range goals {
			id := goals[i].ID
			if status[id] != domain.ProjectStepGateLocked {
				continue
			}
			if !allPrerequisiteGoalsReleased(status, goals[i].DependsOnGoalIDs) {
				continue
			}
			if err := db.Model(&domain.ProjectGoal{}).Where("id = ?", id).Updates(map[string]any{
				"gate_status": string(domain.ProjectStepGateActive),
				"updated_at":  now,
			}).Error; err != nil {
				return err
			}
			status[id] = domain.ProjectStepGateActive
			goals[i].GateStatus = domain.ProjectStepGateActive
			changed = true
		}
		if !changed {
			return nil
		}
	}
	return fmt.Errorf("%w: goal dependency promotion did not stabilize", domain.ErrInvalidInput)
}

func initialGateStatusForNewGoal(deps []string) domain.ProjectStepGateStatus {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.initialGateStatusForNewGoal")
	if len(deps) == 0 {
		return domain.ProjectStepGateActive
	}
	return domain.ProjectStepGateLocked
}

func loadProjectGoalGateGraceSeconds(db *gorm.DB) int {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.loadProjectGoalGateGraceSeconds")
	return loadGateSettings(db).goalGraceSeconds
}

func advanceProjectGoalGateIfReady(tx *gorm.DB, goal *domain.ProjectGoal, graceSeconds int, now time.Time) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.advanceProjectGoalGateIfReady")
	switch goal.GateStatus {
	case domain.ProjectStepGateActive:
		if graceSeconds <= 0 {
			return releaseGoalAndRecalc(tx, goal, now)
		}
		deadline := now.Add(time.Duration(graceSeconds) * time.Second)
		goal.GateStatus = domain.ProjectStepGatePendingRelease
		goal.PendingReleaseDeadlineUTC = &deadline
		goal.UpdatedAt = now
		if err := tx.Save(goal).Error; err != nil {
			return mapWriteError(err)
		}
		gs := loadGateSettings(tx)
		if GateGraceNotify != nil {
			GateGraceNotify.NotifyGoalPendingRelease(context.Background(), goal.ProjectID, goal.ID, deadline.UnixMilli(), gs.goalEmail, gs.goalSMS)
		}
		return nil
	case domain.ProjectStepGatePendingRelease:
		return nil
	default:
		return nil
	}
}

func tryAdvanceGoalGateIfCriteriaComplete(tx *gorm.DB, goalID string, now time.Time) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.tryAdvanceGoalGateIfCriteriaComplete")
	var goal domain.ProjectGoal
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&goal, "id = ?", goalID).Error; err != nil {
		return mapNotFound(err)
	}
	if !domain.GoalCriteriaAllDone(goal.Criteria) {
		return nil
	}
	grace := loadProjectGoalGateGraceSeconds(tx)
	return advanceProjectGoalGateIfReady(tx, &goal, grace, now)
}

func releaseGoalAndRecalc(tx *gorm.DB, goal *domain.ProjectGoal, now time.Time) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.releaseGoalAndRecalc")
	goal.GateStatus = domain.ProjectStepGateReleased
	goal.GateHold = false
	goal.PendingReleaseDeadlineUTC = nil
	goal.UpdatedAt = now
	if err := tx.Save(goal).Error; err != nil {
		return mapWriteError(err)
	}
	if err := recalcLockedGoals(tx, goal.ProjectID, now); err != nil {
		return err
	}
	return recalcLockedSteps(tx, goal.ProjectID, now)
}

// ListProjectGoals returns goals for a project ordered by created_at ascending.
func ListProjectGoals(ctx context.Context, db *gorm.DB, projectID string) ([]domain.ProjectGoal, error) {
	defer kernel.DeferLatency(kernel.OpListProjectGoals)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.ListProjectGoals")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	var rows []domain.ProjectGoal
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list project goals: %w", err)
	}
	return rows, nil
}

// CreateProjectGoal inserts one goal with gate status derived from dependencies.
func CreateProjectGoal(ctx context.Context, db *gorm.DB, projectID string, input CreateProjectGoalInput) (domain.ProjectGoal, error) {
	defer kernel.DeferLatency(kernel.OpCreateProjectGoal)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.CreateProjectGoal")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.ProjectGoal{}, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return domain.ProjectGoal{}, fmt.Errorf("%w: goal title required", domain.ErrInvalidInput)
	}
	var n int64
	if err := db.WithContext(ctx).Model(&domain.Project{}).Where("id = ? AND status = ?", projectID, domain.ProjectStatusActive).Count(&n).Error; err != nil {
		return domain.ProjectGoal{}, fmt.Errorf("project lookup: %w", err)
	}
	if n == 0 {
		return domain.ProjectGoal{}, fmt.Errorf("%w: project not found", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	deps, err := normalizeDependsOnGoalIDs(input.DependsOnGoalIDs, id)
	if err != nil {
		return domain.ProjectGoal{}, err
	}
	for _, dep := range deps {
		var c int64
		if err := db.WithContext(ctx).Model(&domain.ProjectGoal{}).Where("id = ? AND project_id = ?", dep, projectID).Count(&c).Error; err != nil {
			return domain.ProjectGoal{}, fmt.Errorf("prerequisite goal lookup: %w", err)
		}
		if c == 0 {
			return domain.ProjectGoal{}, fmt.Errorf("%w: prerequisite goal not in project", domain.ErrInvalidInput)
		}
	}
	criteria, err := normalizeProjectGoalCriteria(input.Criteria)
	if err != nil {
		return domain.ProjectGoal{}, err
	}
	var existing []domain.ProjectGoal
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).Find(&existing).Error; err != nil {
		return domain.ProjectGoal{}, fmt.Errorf("list goals for cycle check: %w", err)
	}
	candidate := append(existing, domain.ProjectGoal{
		ID:               id,
		ProjectID:        projectID,
		DependsOnGoalIDs: deps,
	})
	if goalDependencyGraphHasCycle(candidate) {
		return domain.ProjectGoal{}, fmt.Errorf("%w: goal dependencies contain a cycle", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	gate := initialGateStatusForNewGoal(deps)
	row := domain.ProjectGoal{
		ID:               id,
		ProjectID:        projectID,
		Title:            title,
		Description:      strings.TrimSpace(input.Description),
		DependsOnGoalIDs: deps,
		GateStatus:       gate,
		GateHold:         false,
		Criteria:         criteria,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ProjectGoal{}, mapWriteError(err)
	}
	if err := recalcLockedGoals(db.WithContext(ctx), projectID, now); err != nil {
		return domain.ProjectGoal{}, err
	}
	if err := recalcLockedSteps(db.WithContext(ctx), projectID, now); err != nil {
		return domain.ProjectGoal{}, err
	}
	var out domain.ProjectGoal
	if err := db.WithContext(ctx).First(&out, "id = ?", id).Error; err != nil {
		return domain.ProjectGoal{}, mapNotFound(err)
	}
	return out, nil
}

// GetProjectGoal loads one goal scoped to projectID.
func GetProjectGoal(ctx context.Context, db *gorm.DB, projectID, goalID string) (domain.ProjectGoal, error) {
	defer kernel.DeferLatency(kernel.OpGetProjectGoal)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.GetProjectGoal")
	projectID = strings.TrimSpace(projectID)
	goalID = strings.TrimSpace(goalID)
	if projectID == "" || goalID == "" {
		return domain.ProjectGoal{}, fmt.Errorf("%w: project id and goal id required", domain.ErrInvalidInput)
	}
	var row domain.ProjectGoal
	if err := db.WithContext(ctx).First(&row, "id = ? AND project_id = ?", goalID, projectID).Error; err != nil {
		return domain.ProjectGoal{}, mapNotFound(err)
	}
	return row, nil
}

// UpdateProjectGoal applies metadata, dependency edits, criteria, and gate_action.
func UpdateProjectGoal(ctx context.Context, db *gorm.DB, projectID, goalID string, input UpdateProjectGoalInput) (domain.ProjectGoal, error) {
	defer kernel.DeferLatency(kernel.OpUpdateProjectGoal)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.UpdateProjectGoal")
	projectID = strings.TrimSpace(projectID)
	goalID = strings.TrimSpace(goalID)
	if projectID == "" || goalID == "" {
		return domain.ProjectGoal{}, fmt.Errorf("%w: project id and goal id required", domain.ErrInvalidInput)
	}
	if input.Title == nil && input.Description == nil && input.DependsOnGoalIDs == nil && input.GateAction == nil && input.Criteria == nil {
		return domain.ProjectGoal{}, fmt.Errorf("%w: no fields to update", domain.ErrInvalidInput)
	}
	var out domain.ProjectGoal
	now := time.Now().UTC()
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row domain.ProjectGoal
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND project_id = ?", goalID, projectID).Error; err != nil {
			return mapNotFound(err)
		}
		if input.Title != nil {
			t := strings.TrimSpace(*input.Title)
			if t == "" {
				return fmt.Errorf("%w: goal title required", domain.ErrInvalidInput)
			}
			row.Title = t
		}
		if input.Description != nil {
			row.Description = strings.TrimSpace(*input.Description)
		}
		if input.DependsOnGoalIDs != nil {
			deps, err := normalizeDependsOnGoalIDs(*input.DependsOnGoalIDs, goalID)
			if err != nil {
				return err
			}
			for _, dep := range deps {
				var c int64
				if err := tx.Model(&domain.ProjectGoal{}).Where("id = ? AND project_id = ?", dep, projectID).Count(&c).Error; err != nil {
					return fmt.Errorf("prerequisite goal lookup: %w", err)
				}
				if c == 0 {
					return fmt.Errorf("%w: prerequisite goal not in project", domain.ErrInvalidInput)
				}
			}
			var all []domain.ProjectGoal
			if err := tx.Where("project_id = ?", projectID).Find(&all).Error; err != nil {
				return err
			}
			for i := range all {
				if all[i].ID == goalID {
					all[i].DependsOnGoalIDs = deps
					break
				}
			}
			if goalDependencyGraphHasCycle(all) {
				return fmt.Errorf("%w: goal dependencies contain a cycle", domain.ErrInvalidInput)
			}
			row.DependsOnGoalIDs = deps
		}
		if input.Criteria != nil {
			criteria, err := normalizeProjectGoalCriteria(*input.Criteria)
			if err != nil {
				return err
			}
			row.Criteria = criteria
		}
		row.UpdatedAt = now
		saved := false
		if input.GateAction != nil {
			act := strings.TrimSpace(strings.ToLower(*input.GateAction))
			switch act {
			case gateActionRelease:
				if err := releaseGoalAndRecalc(tx, &row, now); err != nil {
					return err
				}
				saved = true
			case gateActionHold:
				if row.GateStatus != domain.ProjectStepGatePendingRelease {
					return fmt.Errorf("%w: hold only applies while gate is pending_release", domain.ErrInvalidInput)
				}
				row.GateHold = true
				row.UpdatedAt = now
				if err := tx.Save(&row).Error; err != nil {
					return mapWriteError(err)
				}
				saved = true
			case gateActionClearHold:
				row.GateHold = false
				row.UpdatedAt = now
				deadlinePassed := row.GateStatus == domain.ProjectStepGatePendingRelease &&
					row.PendingReleaseDeadlineUTC != nil && !now.Before(*row.PendingReleaseDeadlineUTC)
				if deadlinePassed {
					if err := releaseGoalAndRecalc(tx, &row, now); err != nil {
						return err
					}
					saved = true
				} else {
					if err := tx.Save(&row).Error; err != nil {
						return mapWriteError(err)
					}
					saved = true
				}
			default:
				return fmt.Errorf("%w: invalid gate_action", domain.ErrInvalidInput)
			}
		}
		if !saved {
			if err := tx.Save(&row).Error; err != nil {
				return mapWriteError(err)
			}
		}
		if err := recalcLockedGoals(tx, projectID, now); err != nil {
			return err
		}
		if err := recalcLockedSteps(tx, projectID, now); err != nil {
			return err
		}
		if input.Criteria != nil {
			if err := tryAdvanceGoalGateIfCriteriaComplete(tx, goalID, now); err != nil {
				return err
			}
		}
		if err := tx.First(&out, "id = ?", goalID).Error; err != nil {
			return mapNotFound(err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ProjectGoal{}, domain.ErrNotFound
		}
		return domain.ProjectGoal{}, err
	}
	return out, nil
}

// DeleteProjectGoal removes a goal when no steps or dependent goals reference it.
func DeleteProjectGoal(ctx context.Context, db *gorm.DB, projectID, goalID string) error {
	defer kernel.DeferLatency(kernel.OpDeleteProjectGoal)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.DeleteProjectGoal")
	projectID = strings.TrimSpace(projectID)
	goalID = strings.TrimSpace(goalID)
	if projectID == "" || goalID == "" {
		return fmt.Errorf("%w: project id and goal id required", domain.ErrInvalidInput)
	}
	var stepCount int64
	if err := db.WithContext(ctx).Model(&domain.ProjectStep{}).Where("goal_id = ?", goalID).Count(&stepCount).Error; err != nil {
		return fmt.Errorf("count steps for goal: %w", err)
	}
	if stepCount > 0 {
		return domain.ErrProjectGoalHasSteps
	}
	var others []domain.ProjectGoal
	if err := db.WithContext(ctx).Where("project_id = ? AND id <> ?", projectID, goalID).Find(&others).Error; err != nil {
		return fmt.Errorf("list goals for dependent check: %w", err)
	}
	for _, g := range others {
		for _, dep := range g.DependsOnGoalIDs {
			if strings.TrimSpace(dep) == goalID {
				return domain.ErrProjectGoalHasDependents
			}
		}
	}
	res := db.WithContext(ctx).Where("id = ? AND project_id = ?", goalID, projectID).Delete(&domain.ProjectGoal{})
	if res.Error != nil {
		return fmt.Errorf("delete project goal: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	if err := recalcLockedGoals(db.WithContext(ctx), projectID, now); err != nil {
		return err
	}
	return recalcLockedSteps(db.WithContext(ctx), projectID, now)
}

// SweepProjectGoalGates auto-releases pending goals past deadline. Returns distinct
// project IDs that had at least one goal updated.
func SweepProjectGoalGates(ctx context.Context, db *gorm.DB, now time.Time) ([]string, error) {
	defer kernel.DeferLatency(kernel.OpSweepProjectGoalGates)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.SweepProjectGoalGates")
	if db == nil {
		return nil, errors.New("tasks store: nil database")
	}
	var due []domain.ProjectGoal
	if err := db.WithContext(ctx).Where("gate_status = ? AND gate_hold = ? AND pending_release_deadline_utc IS NOT NULL AND pending_release_deadline_utc <= ?",
		domain.ProjectStepGatePendingRelease, false, now).Find(&due).Error; err != nil {
		return nil, fmt.Errorf("list pending project goals: %w", err)
	}
	seen := make(map[string]struct{})
	for _, g := range due {
		released := false
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var row domain.ProjectGoal
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", g.ID).Error; err != nil {
				return mapNotFound(err)
			}
			if row.GateStatus != domain.ProjectStepGatePendingRelease || row.GateHold {
				return nil
			}
			if row.PendingReleaseDeadlineUTC == nil || now.Before(*row.PendingReleaseDeadlineUTC) {
				return nil
			}
			if err := releaseGoalAndRecalc(tx, &row, now); err != nil {
				return err
			}
			released = true
			return nil
		}); err != nil {
			return nil, err
		}
		if released {
			seen[g.ProjectID] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out, nil
}
