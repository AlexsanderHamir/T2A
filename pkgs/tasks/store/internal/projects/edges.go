package projects

import (
	"context"
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

// CreateContextEdgeInput is the store input for connecting two project context nodes.
type CreateContextEdgeInput struct {
	ID              string
	SourceContextID string
	TargetContextID string
	Relation        domain.ProjectContextRelation
	Strength        int
	Note            string
}

// UpdateContextEdgeInput is a partial patch for one project context edge.
type UpdateContextEdgeInput struct {
	Relation *domain.ProjectContextRelation
	Strength *int
	Note     *string
}

// CreateContextEdge inserts one relationship between project-owned context nodes.
func CreateContextEdge(ctx context.Context, db *gorm.DB, projectID string, input CreateContextEdgeInput) (domain.ProjectContextEdge, error) {
	defer kernel.DeferLatency(kernel.OpCreateProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.CreateContextEdge")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.ProjectContextEdge{}, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	relation := input.Relation
	if relation == "" {
		relation = domain.ProjectContextRelationRelated
	}
	strength := input.Strength
	if strength == 0 {
		strength = 3
	}
	if err := validateContextEdgeFields(projectID, input.SourceContextID, input.TargetContextID, relation, strength); err != nil {
		return domain.ProjectContextEdge{}, err
	}
	now := time.Now().UTC()
	row := domain.ProjectContextEdge{
		ID:              id,
		ProjectID:       projectID,
		SourceContextID: strings.TrimSpace(input.SourceContextID),
		TargetContextID: strings.TrimSpace(input.TargetContextID),
		Relation:        relation,
		Strength:        strength,
		Note:            strings.TrimSpace(input.Note),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateEdgeNodesExist(tx, projectID, row.SourceContextID, row.TargetContextID); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return mapWriteError(err)
		}
		return nil
	})
	if err != nil {
		return domain.ProjectContextEdge{}, err
	}
	return row, nil
}

// ListContextEdges returns context edges for one project, optionally restricted to a selected node set.
func ListContextEdges(ctx context.Context, db *gorm.DB, projectID string, nodeIDs []string) ([]domain.ProjectContextEdge, error) {
	defer kernel.DeferLatency(kernel.OpListProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.ListContextEdges")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project id required", domain.ErrInvalidInput)
	}
	q := db.WithContext(ctx).Where("project_id = ?", projectID).Order("updated_at DESC")
	if nodeIDs != nil {
		ids := normalizeEdgeNodeFilter(nodeIDs)
		if len(ids) == 0 {
			return nil, nil
		}
		q = q.Where("source_context_id IN ? AND target_context_id IN ?", ids, ids)
	}
	var rows []domain.ProjectContextEdge
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list project context edges: %w", err)
	}
	return rows, nil
}

// UpdateContextEdge applies a partial patch to one context edge.
func UpdateContextEdge(ctx context.Context, db *gorm.DB, projectID, edgeID string, input UpdateContextEdgeInput) (domain.ProjectContextEdge, error) {
	defer kernel.DeferLatency(kernel.OpUpdateProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.UpdateContextEdge")
	projectID = strings.TrimSpace(projectID)
	edgeID = strings.TrimSpace(edgeID)
	if projectID == "" || edgeID == "" {
		return domain.ProjectContextEdge{}, fmt.Errorf("%w: project id and edge id required", domain.ErrInvalidInput)
	}
	if err := validateContextEdgePatch(input); err != nil {
		return domain.ProjectContextEdge{}, err
	}
	var out domain.ProjectContextEdge
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row domain.ProjectContextEdge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND project_id = ?", edgeID, projectID).Error; err != nil {
			return mapNotFound(err)
		}
		applyContextEdgePatch(&row, input)
		row.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&row).Error; err != nil {
			return mapWriteError(err)
		}
		out = row
		return nil
	})
	if err != nil {
		return domain.ProjectContextEdge{}, err
	}
	return out, nil
}

// DeleteContextEdge removes one relationship between project context nodes.
func DeleteContextEdge(ctx context.Context, db *gorm.DB, projectID, edgeID string) error {
	defer kernel.DeferLatency(kernel.OpDeleteProjectContext)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.projects.DeleteContextEdge")
	projectID = strings.TrimSpace(projectID)
	edgeID = strings.TrimSpace(edgeID)
	if projectID == "" || edgeID == "" {
		return fmt.Errorf("%w: project id and edge id required", domain.ErrInvalidInput)
	}
	res := db.WithContext(ctx).Where("id = ? AND project_id = ?", edgeID, projectID).Delete(&domain.ProjectContextEdge{})
	if res.Error != nil {
		return mapWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func validateContextEdgeFields(projectID, sourceID, targetID string, relation domain.ProjectContextRelation, strength int) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(sourceID) == "" || strings.TrimSpace(targetID) == "" {
		return fmt.Errorf("%w: project id, source_context_id, and target_context_id required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(sourceID) == strings.TrimSpace(targetID) {
		return fmt.Errorf("%w: context edge cannot connect a node to itself", domain.ErrInvalidInput)
	}
	if err := validateContextRelation(relation); err != nil {
		return err
	}
	if strength < 1 || strength > 5 {
		return fmt.Errorf("%w: strength must be 1..5", domain.ErrInvalidInput)
	}
	return nil
}

func validateContextEdgePatch(input UpdateContextEdgeInput) error {
	if input.Relation != nil {
		if err := validateContextRelation(*input.Relation); err != nil {
			return err
		}
	}
	if input.Strength != nil && (*input.Strength < 1 || *input.Strength > 5) {
		return fmt.Errorf("%w: strength must be 1..5", domain.ErrInvalidInput)
	}
	return nil
}

func validateContextRelation(relation domain.ProjectContextRelation) error {
	switch relation {
	case domain.ProjectContextRelationSupports,
		domain.ProjectContextRelationBlocks,
		domain.ProjectContextRelationRefines,
		domain.ProjectContextRelationDependsOn,
		domain.ProjectContextRelationRelated:
		return nil
	default:
		return fmt.Errorf("%w: invalid context relation %q", domain.ErrInvalidInput, relation)
	}
}

func validateEdgeNodesExist(tx *gorm.DB, projectID, sourceID, targetID string) error {
	var count int64
	if err := tx.Model(&domain.ProjectContextItem{}).
		Where("project_id = ? AND id IN ?", projectID, []string{sourceID, targetID}).
		Count(&count).Error; err != nil {
		return fmt.Errorf("context edge node lookup: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("%w: context edge nodes must belong to project", domain.ErrInvalidInput)
	}
	return nil
}

func applyContextEdgePatch(row *domain.ProjectContextEdge, input UpdateContextEdgeInput) {
	if input.Relation != nil {
		row.Relation = *input.Relation
	}
	if input.Strength != nil {
		row.Strength = *input.Strength
	}
	if input.Note != nil {
		row.Note = strings.TrimSpace(*input.Note)
	}
}

func normalizeEdgeNodeFilter(nodeIDs []string) []string {
	out := make([]string, 0, len(nodeIDs))
	seen := make(map[string]bool, len(nodeIDs))
	for _, value := range nodeIDs {
		id := strings.TrimSpace(value)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
