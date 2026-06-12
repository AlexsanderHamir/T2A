package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/checklist"
)

func TestAddDependency_rejectsSelfAndCycle(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()

	a, err := Create(ctx, db, CreateInput{Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Create(ctx, db, CreateInput{Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Create(ctx, db, CreateInput{Title: "c", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	if err := AddDependency(ctx, db, a.ID, a.ID); err == nil || !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("self dep: %v", err)
	}
	if err := AddDependency(ctx, db, b.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(ctx, db, c.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(ctx, db, a.ID, c.ID); err == nil || !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("cycle dep: %v", err)
	}
}

func TestCreateTask_rejectsSubtaskOfSubtask(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()

	root, err := Create(ctx, db, CreateInput{Title: "root", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checklist.Add(ctx, db, root.ID, "parent criterion", domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	child, err := Create(ctx, db, CreateInput{Title: "child", Priority: domain.PriorityMedium, ParentID: &root.ID}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Create(ctx, db, CreateInput{Title: "grand", Priority: domain.PriorityMedium, ParentID: &child.ID}, domain.ActorUser)
	if err == nil || !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("grandchild create: %v", err)
	}
}

func TestGet_hydratesDependsOn(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()

	a, err := Create(ctx, db, CreateInput{Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Create(ctx, db, CreateInput{Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(ctx, db, b.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	got, err := Get(ctx, db, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 1 || got.DependsOn[0] != a.ID {
		t.Fatalf("depends_on: %#v", got.DependsOn)
	}
}
