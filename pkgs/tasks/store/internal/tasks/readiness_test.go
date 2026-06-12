package tasks

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/ready"
)

func TestListQueueCandidates_excludesOpenDependency(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()

	dep, err := Create(ctx, db, CreateInput{Title: "dep", Status: domain.StatusReady, Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := Create(ctx, db, CreateInput{Title: "blocked", Status: domain.StatusReady, Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(ctx, db, blocked.ID, dep.ID, domain.DependencySatisfiesDone); err != nil {
		t.Fatal(err)
	}

	cands, err := ready.ListQueueCandidates(ctx, db, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsCandidateID(cands, blocked.ID) {
		t.Fatalf("blocked task %q should not be a queue candidate", blocked.ID)
	}

	done := domain.StatusDone
	if _, _, err := Update(ctx, db, dep.ID, UpdateInput{Status: &done}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	cands, err = ready.ListQueueCandidates(ctx, db, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsCandidateID(cands, blocked.ID) {
		t.Fatalf("blocked task %q should dequeue after dependency is done", blocked.ID)
	}
}

func TestListQueueCandidates_excludesHeldGate(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()

	heldGate := &domain.TaskGate{
		Kind:   domain.GateKindManualApproval,
		Status: domain.GateStatusActive,
	}
	held, err := Create(ctx, db, CreateInput{
		Title:    "held",
		Status:   domain.StatusReady,
		Priority: domain.PriorityMedium,
		Gate:     heldGate,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	cands, err := ready.ListQueueCandidates(ctx, db, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsCandidateID(cands, held.ID) {
		t.Fatalf("gated task %q should not be a queue candidate", held.ID)
	}

	released := &domain.TaskGate{
		Kind:   domain.GateKindManualApproval,
		Status: domain.GateStatusReleased,
	}
	gatePtr := released
	if _, _, err := Update(ctx, db, held.ID, UpdateInput{Gate: &gatePtr}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	cands, err = ready.ListQueueCandidates(ctx, db, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsCandidateID(cands, held.ID) {
		t.Fatalf("released task %q should be a queue candidate", held.ID)
	}
}

func TestListFlat_filterByTagAndMilestone(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()

	tag := "infra"
	milestone := "m1"
	if _, err := Create(ctx, db, CreateInput{
		Title:     "tagged",
		Status:    domain.StatusBlocked,
		Priority:  domain.PriorityMedium,
		Tags:      []string{tag, "other"},
		Milestone: &milestone,
	}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(ctx, db, CreateInput{
		Title:    "plain",
		Status:   domain.StatusBlocked,
		Priority: domain.PriorityMedium,
	}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}

	out, err := ListFlat(ctx, db, 50, 0, &ListFilter{Tag: &tag})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Title != "tagged" {
		t.Fatalf("tag filter: got %+v", out)
	}

	ms := milestone
	out, err = ListFlat(ctx, db, 50, 0, &ListFilter{Milestone: &ms})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Title != "tagged" {
		t.Fatalf("milestone filter: got %+v", out)
	}
}

func containsCandidateID(cands []ready.QueueCandidate, id string) bool {
	for _, c := range cands {
		if c.Task.ID == id {
			return true
		}
	}
	return false
}
