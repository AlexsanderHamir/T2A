// Package projects owns persistence for first-class projects, curated project
// context items, and immutable task context snapshots.
package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const logCmd = "taskapi"

// CreateProjectInput is the store input for creating a project.
type CreateProjectInput struct {
	ID             string
	Name           string
	Description    string
	ContextSummary string
}

// UpdateProjectInput is a partial patch for project metadata.
type UpdateProjectInput struct {
	Name           *string
	Description    *string
	Status         *domain.ProjectStatus
	ContextSummary *string
}

// CreateContextInput is the store input for appending a project context item.
type CreateContextInput struct {
	ID            string
	Kind          domain.ProjectContextKind
	Title         string
	Body          string
	SourceTaskID  *string
	SourceCycleID *string
	CreatedBy     domain.Actor
	Pinned        bool
}

// UpdateContextInput is a partial patch for one project context item.
type UpdateContextInput struct {
	Kind   *domain.ProjectContextKind
	Title  *string
	Body   *string
	Pinned *bool
}

// CreateSnapshotInput records the exact context bundle handed to one cycle.
type CreateSnapshotInput struct {
	ID              string
	TaskID          string
	CycleID         string
	ProjectID       string
	ContextJSON     json.RawMessage
	RenderedContext string
	TokenEstimate   int
}

// CreateProject inserts a new active project.
func CreateProject(ctx context.Context, db *gorm.DB, input CreateProjectInput) (domain.Project, error) {
	defer kernel.DeferLatency(kernel.OpCreateProject)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.CreateProject")
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Project{}, fmt.Errorf("%w: project name required", domain.ErrInvalidInput)
	}
	now := time.Now().UTC()
	row := domain.Project{
		ID:             id,
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		Status:         domain.ProjectStatusActive,
		ContextSummary: strings.TrimSpace(input.ContextSummary),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Project{}, mapWriteError(err)
	}
	return row, nil
}

// ListProjects returns projects ordered by most recently updated first.
func ListProjects(ctx context.Context, db *gorm.DB, includeArchived bool, limit int) ([]domain.Project, error) {
	defer kernel.DeferLatency(kernel.OpListProjects)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.ListProjects")
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	q := db.WithContext(ctx).Order("updated_at DESC").Limit(limit)
	if !includeArchived {
		q = q.Where("status = ?", domain.ProjectStatusActive)
	}
	var rows []domain.Project
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return rows, nil
}

// GetProject returns one project by id.
func GetProject(ctx context.Context, db *gorm.DB, id string) (domain.Project, error) {
	defer kernel.DeferLatency(kernel.OpGetProject)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.GetProject")
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Project{}, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	var row domain.Project
	if err := db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Project{}, mapNotFound(err)
	}
	return row, nil
}

// UpdateProject applies a partial metadata patch and returns the updated row.
func UpdateProject(ctx context.Context, db *gorm.DB, id string, input UpdateProjectInput) (domain.Project, error) {
	defer kernel.DeferLatency(kernel.OpUpdateProject)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.UpdateProject")
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Project{}, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	if err := validateProjectPatch(input); err != nil {
		return domain.Project{}, err
	}
	var out domain.Project
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row domain.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return mapNotFound(err)
		}
		if err := validateDefaultProjectPatch(row, input); err != nil {
			return err
		}
		applyProjectPatch(&row, input)
		row.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&row).Error; err != nil {
			return mapWriteError(err)
		}
		out = row
		return nil
	})
	if err != nil {
		return domain.Project{}, err
	}
	return out, nil
}

// DeleteProject removes a project when no tasks still reference it.
func DeleteProject(ctx context.Context, db *gorm.DB, id string) error {
	defer kernel.DeferLatency(kernel.OpDeleteProject)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.DeleteProject")
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	if id == domain.DefaultProjectID {
		return fmt.Errorf("%w: default project cannot be deleted", domain.ErrConflict)
	}
	var taskCount int64
	if err := db.WithContext(ctx).Model(&domain.Task{}).Where("project_id = ?", id).Count(&taskCount).Error; err != nil {
		return fmt.Errorf("count project tasks: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("%w: project has tasks", domain.ErrConflict)
	}
	res := db.WithContext(ctx).Delete(&domain.Project{}, "id = ?", id)
	if res.Error != nil {
		return mapWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CreateContext inserts one context item for a project.
func CreateContext(ctx context.Context, db *gorm.DB, projectID string, input CreateContextInput) (domain.ProjectContextItem, error) {
	defer kernel.DeferLatency(kernel.OpCreateProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.CreateContext")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.ProjectContextItem{}, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	kind := domain.ProjectContextKind(strings.TrimSpace(string(input.Kind)))
	if kind == "" {
		kind = domain.ProjectContextKindNote
	}
	if err := validateContextKind(kind); err != nil {
		return domain.ProjectContextItem{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return domain.ProjectContextItem{}, fmt.Errorf("%w: context title required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(input.Body) == "" {
		return domain.ProjectContextItem{}, fmt.Errorf("%w: context body required", domain.ErrInvalidInput)
	}
	actor := input.CreatedBy
	if actor == "" {
		actor = domain.ActorUser
	}
	if err := kernel.ValidateActor(actor); err != nil {
		return domain.ProjectContextItem{}, err
	}
	now := time.Now().UTC()
	row := domain.ProjectContextItem{
		ID:            id,
		ProjectID:     projectID,
		Kind:          kind,
		Title:         title,
		Body:          strings.TrimSpace(input.Body),
		SourceTaskID:  trimOptional(input.SourceTaskID),
		SourceCycleID: trimOptional(input.SourceCycleID),
		CreatedBy:     actor,
		Pinned:        input.Pinned,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ProjectContextItem{}, mapWriteError(err)
	}
	return row, nil
}

// ListContext returns context items for a project, pinned items first.
func ListContext(ctx context.Context, db *gorm.DB, projectID string, includeUnpinned bool, limit int) ([]domain.ProjectContextItem, error) {
	defer kernel.DeferLatency(kernel.OpListProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.ListContext")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	q := db.WithContext(ctx).Where("project_id = ?", projectID).Order("pinned DESC").Order("updated_at DESC").Limit(limit)
	if !includeUnpinned {
		q = q.Where("pinned = ?", true)
	}
	var rows []domain.ProjectContextItem
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list project context: %w", err)
	}
	return rows, nil
}

// ListContextByIDs returns selected context items for one project in caller order.
func ListContextByIDs(ctx context.Context, db *gorm.DB, projectID string, ids []string) ([]domain.ProjectContextItem, error) {
	defer kernel.DeferLatency(kernel.OpListProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.ListContextByIDs")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []domain.ProjectContextItem
	if err := db.WithContext(ctx).Where("project_id = ? AND id IN ?", projectID, ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list selected project context: %w", err)
	}
	byID := make(map[string]domain.ProjectContextItem, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	out := make([]domain.ProjectContextItem, 0, len(ids))
	for _, id := range ids {
		row, ok := byID[strings.TrimSpace(id)]
		if !ok {
			return nil, domain.ErrNotFound
		}
		out = append(out, row)
	}
	return out, nil
}

// UpdateContext applies a partial patch to one context item.
func UpdateContext(ctx context.Context, db *gorm.DB, projectID, itemID string, input UpdateContextInput) (domain.ProjectContextItem, error) {
	defer kernel.DeferLatency(kernel.OpUpdateProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.UpdateContext")
	projectID = strings.TrimSpace(projectID)
	itemID = strings.TrimSpace(itemID)
	if projectID == "" || itemID == "" {
		return domain.ProjectContextItem{}, fmt.Errorf("%w: project id and context id required", domain.ErrInvalidInput)
	}
	if err := validateContextPatch(input); err != nil {
		return domain.ProjectContextItem{}, err
	}
	var out domain.ProjectContextItem
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row domain.ProjectContextItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND project_id = ?", itemID, projectID).Error; err != nil {
			return mapNotFound(err)
		}
		applyContextPatch(&row, input)
		row.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&row).Error; err != nil {
			return mapWriteError(err)
		}
		out = row
		return nil
	})
	if err != nil {
		return domain.ProjectContextItem{}, err
	}
	return out, nil
}

// DeleteContext removes one context item.
func DeleteContext(ctx context.Context, db *gorm.DB, projectID, itemID string) error {
	defer kernel.DeferLatency(kernel.OpDeleteProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.DeleteContext")
	projectID = strings.TrimSpace(projectID)
	itemID = strings.TrimSpace(itemID)
	if projectID == "" || itemID == "" {
		return fmt.Errorf("%w: project id and context id required", domain.ErrInvalidInput)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ? AND (source_context_id = ? OR target_context_id = ?)", projectID, itemID, itemID).Delete(&domain.ProjectContextEdge{}).Error; err != nil {
			return mapWriteError(err)
		}
		res := tx.Where("id = ? AND project_id = ?", itemID, projectID).Delete(&domain.ProjectContextItem{})
		if res.Error != nil {
			return mapWriteError(res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// CreateSnapshot inserts an immutable task context snapshot.
func CreateSnapshot(ctx context.Context, db *gorm.DB, input CreateSnapshotInput) (domain.TaskContextSnapshot, error) {
	defer kernel.DeferLatency(kernel.OpCreateContextSnapshot)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.CreateSnapshot")
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	if strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.CycleID) == "" || strings.TrimSpace(input.ProjectID) == "" {
		return domain.TaskContextSnapshot{}, fmt.Errorf("%w: task_id, cycle_id, and project_id required", domain.ErrInvalidInput)
	}
	if input.TokenEstimate < 0 {
		return domain.TaskContextSnapshot{}, fmt.Errorf("%w: token_estimate must be >= 0", domain.ErrInvalidInput)
	}
	contextJSON, err := kernel.NormalizeJSONObject(input.ContextJSON, "context_json")
	if err != nil {
		return domain.TaskContextSnapshot{}, err
	}
	row := domain.TaskContextSnapshot{
		ID:              id,
		TaskID:          strings.TrimSpace(input.TaskID),
		CycleID:         strings.TrimSpace(input.CycleID),
		ProjectID:       strings.TrimSpace(input.ProjectID),
		ContextJSON:     datatypes.JSON(contextJSON),
		RenderedContext: strings.TrimSpace(input.RenderedContext),
		TokenEstimate:   input.TokenEstimate,
		CreatedAt:       time.Now().UTC(),
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.TaskContextSnapshot{}, mapWriteError(err)
	}
	return row, nil
}

// GetSnapshotForCycle returns the context snapshot recorded for a cycle.
func GetSnapshotForCycle(ctx context.Context, db *gorm.DB, cycleID string) (domain.TaskContextSnapshot, error) {
	defer kernel.DeferLatency(kernel.OpGetContextSnapshot)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.GetSnapshotForCycle")
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return domain.TaskContextSnapshot{}, fmt.Errorf("%w: cycle id required", domain.ErrInvalidInput)
	}
	var row domain.TaskContextSnapshot
	if err := db.WithContext(ctx).First(&row, "cycle_id = ?", cycleID).Error; err != nil {
		return domain.TaskContextSnapshot{}, mapNotFound(err)
	}
	return row, nil
}

func validateProjectPatch(input UpdateProjectInput) error {
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return fmt.Errorf("%w: project name required", domain.ErrInvalidInput)
	}
	if input.Status != nil {
		switch *input.Status {
		case domain.ProjectStatusActive, domain.ProjectStatusArchived:
		default:
			return fmt.Errorf("%w: invalid project status %q", domain.ErrInvalidInput, *input.Status)
		}
	}
	return nil
}

func validateDefaultProjectPatch(row domain.Project, input UpdateProjectInput) error {
	if row.ID != domain.DefaultProjectID {
		return nil
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) != domain.DefaultProject(time.Now()).Name {
		return fmt.Errorf("%w: default project name cannot be changed", domain.ErrConflict)
	}
	if input.Status != nil && *input.Status != domain.ProjectStatusActive {
		return fmt.Errorf("%w: default project cannot be archived", domain.ErrConflict)
	}
	return nil
}

func applyProjectPatch(row *domain.Project, input UpdateProjectInput) {
	if input.Name != nil {
		row.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		row.Description = strings.TrimSpace(*input.Description)
	}
	if input.Status != nil {
		row.Status = *input.Status
	}
	if input.ContextSummary != nil {
		row.ContextSummary = strings.TrimSpace(*input.ContextSummary)
	}
}

func validateContextPatch(input UpdateContextInput) error {
	if input.Kind != nil {
		if err := validateContextKind(*input.Kind); err != nil {
			return err
		}
	}
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return fmt.Errorf("%w: context title required", domain.ErrInvalidInput)
	}
	if input.Body != nil && strings.TrimSpace(*input.Body) == "" {
		return fmt.Errorf("%w: context body required", domain.ErrInvalidInput)
	}
	return nil
}

func applyContextPatch(row *domain.ProjectContextItem, input UpdateContextInput) {
	if input.Kind != nil {
		row.Kind = domain.ProjectContextKind(strings.TrimSpace(string(*input.Kind)))
	}
	if input.Title != nil {
		row.Title = strings.TrimSpace(*input.Title)
	}
	if input.Body != nil {
		row.Body = strings.TrimSpace(*input.Body)
	}
	if input.Pinned != nil {
		row.Pinned = *input.Pinned
	}
}

func validateContextKind(kind domain.ProjectContextKind) error {
	trimmed := strings.TrimSpace(string(kind))
	if trimmed == "" {
		return fmt.Errorf("%w: context kind required", domain.ErrInvalidInput)
	}
	if len(trimmed) > 24 {
		return fmt.Errorf("%w: context kind must be 24 characters or fewer", domain.ErrInvalidInput)
	}
	return nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("db: %w", err)
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique constraint") {
		return fmt.Errorf("%w: duplicate project row", domain.ErrConflict)
	}
	if strings.Contains(msg, "constraint failed") || strings.Contains(msg, "foreign key") || strings.Contains(msg, "violates check constraint") {
		return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	return fmt.Errorf("db: %w", err)
}
