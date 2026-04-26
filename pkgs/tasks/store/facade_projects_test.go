package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func newProjectStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	return NewStore(tasktestdb.OpenSQLite(t)), context.Background()
}

func TestStore_ProjectCRUD_roundtrip(t *testing.T) {
	s, ctx := newProjectStore(t)

	project, err := s.CreateProject(ctx, CreateProjectInput{
		Name:           "Project moat",
		Description:    "Long-running project context",
		ContextSummary: "Shared memory for related tasks",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.ID == "" {
		t.Fatal("expected generated project id")
	}
	if project.Status != domain.ProjectStatusActive {
		t.Fatalf("status = %q, want active", project.Status)
	}

	got, err := s.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.Name != "Project moat" || got.ContextSummary == "" {
		t.Fatalf("project = %#v", got)
	}

	archived := domain.ProjectStatusArchived
	renamed := "Project context moat"
	updated, err := s.UpdateProject(ctx, project.ID, UpdateProjectInput{
		Name:   &renamed,
		Status: &archived,
	})
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if updated.Name != renamed || updated.Status != archived {
		t.Fatalf("updated = %#v", updated)
	}

	active, err := s.ListProjects(ctx, false, 10)
	if err != nil {
		t.Fatalf("list active projects: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active projects = %#v, want none", active)
	}

	all, err := s.ListProjects(ctx, true, 10)
	if err != nil {
		t.Fatalf("list all projects: %v", err)
	}
	if len(all) != 1 || all[0].ID != project.ID {
		t.Fatalf("all projects = %#v", all)
	}

	if err := s.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	_, err = s.GetProject(ctx, project.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get deleted err = %v, want ErrNotFound", err)
	}
}

func TestStore_ProjectContextCRUD_roundtrip(t *testing.T) {
	s, ctx := newProjectStore(t)
	project, err := s.CreateProject(ctx, CreateProjectInput{Name: "Context project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	first, err := s.CreateProjectContext(ctx, project.ID, CreateProjectContextInput{
		Kind:      domain.ProjectContextKindDecision,
		Title:     "Use relational memory first",
		Body:      "Defer embeddings until explicit context works.",
		CreatedBy: domain.ActorUser,
		Pinned:    true,
	})
	if err != nil {
		t.Fatalf("create first context: %v", err)
	}
	second, err := s.CreateProjectContext(ctx, project.ID, CreateProjectContextInput{
		Kind:      domain.ProjectContextKindNote,
		Title:     "Loose note",
		Body:      "Visible but not pinned.",
		CreatedBy: domain.ActorAgent,
	})
	if err != nil {
		t.Fatalf("create second context: %v", err)
	}

	pinned, err := s.ListProjectContext(ctx, project.ID, false, 10)
	if err != nil {
		t.Fatalf("list pinned context: %v", err)
	}
	if len(pinned) != 1 || pinned[0].ID != first.ID {
		t.Fatalf("pinned context = %#v", pinned)
	}

	all, err := s.ListProjectContext(ctx, project.ID, true, 10)
	if err != nil {
		t.Fatalf("list all context: %v", err)
	}
	if len(all) != 2 || all[0].ID != first.ID {
		t.Fatalf("all context = %#v", all)
	}

	pinSecond := true
	kind := domain.ProjectContextKindHandoff
	updated, err := s.UpdateProjectContext(ctx, project.ID, second.ID, UpdateProjectContextInput{
		Kind:   &kind,
		Pinned: &pinSecond,
	})
	if err != nil {
		t.Fatalf("update context: %v", err)
	}
	if updated.Kind != kind || !updated.Pinned {
		t.Fatalf("updated context = %#v", updated)
	}

	if err := s.DeleteProjectContext(ctx, project.ID, first.ID); err != nil {
		t.Fatalf("delete context: %v", err)
	}
	remaining, err := s.ListProjectContext(ctx, project.ID, true, 10)
	if err != nil {
		t.Fatalf("list remaining context: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != second.ID {
		t.Fatalf("remaining context = %#v", remaining)
	}
}

func TestStore_TaskContextSnapshot_roundtrip(t *testing.T) {
	s, ctx := newProjectStore(t)
	project, err := s.CreateProject(ctx, CreateProjectInput{Name: "Snapshot project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := mustCreateTask(t, s, ctx)
	cycle, err := s.StartCycle(ctx, StartCycleInput{TaskID: task.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}

	raw := json.RawMessage(`{"project_id":"` + project.ID + `","items":[{"id":"ctx-1"}]}`)
	snapshot, err := s.CreateTaskContextSnapshot(ctx, CreateTaskContextSnapshotInput{
		TaskID:          task.ID,
		CycleID:         cycle.ID,
		ProjectID:       project.ID,
		ContextJSON:     raw,
		RenderedContext: "## Project context\n- Use relational memory first.",
		TokenEstimate:   42,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if snapshot.ID == "" {
		t.Fatal("expected generated snapshot id")
	}

	got, err := s.GetTaskContextSnapshotForCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if got.TaskID != task.ID || got.ProjectID != project.ID || got.TokenEstimate != 42 {
		t.Fatalf("snapshot = %#v", got)
	}
	if string(got.ContextJSON) != string(raw) {
		t.Fatalf("context_json = %s, want %s", string(got.ContextJSON), string(raw))
	}
}

func TestStore_Project_validation_errors(t *testing.T) {
	s, ctx := newProjectStore(t)

	if _, err := s.CreateProject(ctx, CreateProjectInput{Name: " "}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create empty name err = %v, want ErrInvalidInput", err)
	}

	project, err := s.CreateProject(ctx, CreateProjectInput{Name: "Validation project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	badKind := domain.ProjectContextKind("memory")
	if _, err := s.CreateProjectContext(ctx, project.ID, CreateProjectContextInput{
		Kind:      badKind,
		Title:     "Bad",
		Body:      "Bad",
		CreatedBy: domain.ActorUser,
	}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("create bad kind err = %v, want ErrInvalidInput", err)
	}

	_, err = s.GetProject(ctx, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get missing project err = %v, want ErrNotFound", err)
	}
}
