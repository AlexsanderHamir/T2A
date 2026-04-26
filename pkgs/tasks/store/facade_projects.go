package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/projects"
)

// CreateProjectInput is the store input for creating a project.
type CreateProjectInput = projects.CreateProjectInput

// UpdateProjectInput is a partial patch for project metadata.
type UpdateProjectInput = projects.UpdateProjectInput

// CreateProjectContextInput is the store input for appending a project context item.
type CreateProjectContextInput = projects.CreateContextInput

// UpdateProjectContextInput is a partial patch for a project context item.
type UpdateProjectContextInput = projects.UpdateContextInput

// CreateProjectContextEdgeInput is the store input for connecting context nodes.
type CreateProjectContextEdgeInput = projects.CreateContextEdgeInput

// UpdateProjectContextEdgeInput is a partial patch for a project context edge.
type UpdateProjectContextEdgeInput = projects.UpdateContextEdgeInput

// CreateTaskContextSnapshotInput records the rendered project context passed to a cycle.
type CreateTaskContextSnapshotInput = projects.CreateSnapshotInput

// CreateProject inserts a new active project.
func (s *Store) CreateProject(ctx context.Context, input CreateProjectInput) (domain.Project, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CreateProject")
	return projects.CreateProject(ctx, s.db, input)
}

// ListProjects returns projects ordered by most recently updated first.
func (s *Store) ListProjects(ctx context.Context, includeArchived bool, limit int) ([]domain.Project, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListProjects")
	return projects.ListProjects(ctx, s.db, includeArchived, limit)
}

// GetProject returns one project by id.
func (s *Store) GetProject(ctx context.Context, id string) (domain.Project, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetProject")
	return projects.GetProject(ctx, s.db, id)
}

// UpdateProject applies a partial project metadata patch.
func (s *Store) UpdateProject(ctx context.Context, id string, input UpdateProjectInput) (domain.Project, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateProject")
	return projects.UpdateProject(ctx, s.db, id, input)
}

// DeleteProject removes a project when no tasks still reference it.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteProject")
	return projects.DeleteProject(ctx, s.db, id)
}

// CreateProjectContext inserts one context item for a project.
func (s *Store) CreateProjectContext(ctx context.Context, projectID string, input CreateProjectContextInput) (domain.ProjectContextItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CreateProjectContext")
	return projects.CreateContext(ctx, s.db, projectID, input)
}

// ListProjectContext returns context items for a project, pinned items first.
func (s *Store) ListProjectContext(ctx context.Context, projectID string, includeUnpinned bool, limit int) ([]domain.ProjectContextItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListProjectContext")
	return projects.ListContext(ctx, s.db, projectID, includeUnpinned, limit)
}

// ListProjectContextByIDs returns selected context items in caller order.
func (s *Store) ListProjectContextByIDs(ctx context.Context, projectID string, ids []string) ([]domain.ProjectContextItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListProjectContextByIDs")
	return projects.ListContextByIDs(ctx, s.db, projectID, ids)
}

// CreateProjectContextEdge inserts one relationship between project context nodes.
func (s *Store) CreateProjectContextEdge(ctx context.Context, projectID string, input CreateProjectContextEdgeInput) (domain.ProjectContextEdge, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CreateProjectContextEdge")
	return projects.CreateContextEdge(ctx, s.db, projectID, input)
}

// ListProjectContextEdges returns context edges for one project.
func (s *Store) ListProjectContextEdges(ctx context.Context, projectID string, nodeIDs []string) ([]domain.ProjectContextEdge, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListProjectContextEdges")
	return projects.ListContextEdges(ctx, s.db, projectID, nodeIDs)
}

// UpdateProjectContextEdge applies a partial patch to one project context edge.
func (s *Store) UpdateProjectContextEdge(ctx context.Context, projectID, edgeID string, input UpdateProjectContextEdgeInput) (domain.ProjectContextEdge, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateProjectContextEdge")
	return projects.UpdateContextEdge(ctx, s.db, projectID, edgeID, input)
}

// DeleteProjectContextEdge removes one relationship between project context nodes.
func (s *Store) DeleteProjectContextEdge(ctx context.Context, projectID, edgeID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteProjectContextEdge")
	return projects.DeleteContextEdge(ctx, s.db, projectID, edgeID)
}

// UpdateProjectContext applies a partial patch to one project context item.
func (s *Store) UpdateProjectContext(ctx context.Context, projectID, itemID string, input UpdateProjectContextInput) (domain.ProjectContextItem, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpdateProjectContext")
	return projects.UpdateContext(ctx, s.db, projectID, itemID, input)
}

// DeleteProjectContext removes one project context item.
func (s *Store) DeleteProjectContext(ctx context.Context, projectID, itemID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.DeleteProjectContext")
	return projects.DeleteContext(ctx, s.db, projectID, itemID)
}

// CreateTaskContextSnapshot inserts an immutable project-context snapshot for a cycle.
func (s *Store) CreateTaskContextSnapshot(ctx context.Context, input CreateTaskContextSnapshotInput) (domain.TaskContextSnapshot, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CreateTaskContextSnapshot")
	return projects.CreateSnapshot(ctx, s.db, input)
}

// GetTaskContextSnapshotForCycle returns the context snapshot recorded for a cycle.
func (s *Store) GetTaskContextSnapshotForCycle(ctx context.Context, cycleID string) (domain.TaskContextSnapshot, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetTaskContextSnapshotForCycle")
	return projects.GetSnapshotForCycle(ctx, s.db, cycleID)
}
