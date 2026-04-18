package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestStore_StartCycle_meta_normalizes_null asserts the documented invariant
// for task_cycles.meta_json (docs/EXECUTION-CYCLES.md §column conventions and
// docs/API-HTTP.md POST /tasks/{id}/cycles): the column never carries a
// non-object JSON value. The store must normalize the JSON literal "null"
// (semantically equivalent to "no meta provided") to the canonical "{}"
// rather than persisting the literal "null", which the API doc promises
// will never appear in responses.
func TestStore_StartCycle_meta_normalizes_null(t *testing.T) {
	s, ctx := newCycleStoreMV(t)
	tsk := mustCreateTaskMV(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{
		TaskID:      tsk.ID,
		TriggeredBy: domain.ActorAgent,
		Meta:        []byte("null"),
	})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if string(c.MetaJSON) != "{}" {
		t.Fatalf("meta_json = %q, want {} (null must normalize per documented invariant)", string(c.MetaJSON))
	}
}

// TestStore_StartCycle_meta_rejects_non_object_json asserts that meta values
// that are valid JSON but not objects (string, number, array, bool) are
// rejected as ErrInvalidInput. The contract is "opaque JSON object" — silent
// coercion would let the column hold values that violate the documented
// shape, leading to client parsers that crash on the wire later.
func TestStore_StartCycle_meta_rejects_non_object_json(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"string", []byte(`"foo"`)},
		{"number", []byte(`123`)},
		{"array", []byte(`[1,2,3]`)},
		{"bool", []byte(`true`)},
		{"malformed", []byte(`{not-json`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx := newCycleStoreMV(t)
			tsk := mustCreateTaskMV(t, s, ctx)
			_, err := s.StartCycle(ctx, StartCycleInput{
				TaskID:      tsk.ID,
				TriggeredBy: domain.ActorAgent,
				Meta:        tc.body,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput for meta=%s", err, string(tc.body))
			}
		})
	}
}

// TestStore_StartCycle_meta_passes_through_object asserts that a well-formed
// JSON object meta survives the normalize step unchanged (byte-equal, after
// the json package's canonical form). This pins the happy-path contract so
// the validation logic above does not regress into rejecting valid input.
func TestStore_StartCycle_meta_passes_through_object(t *testing.T) {
	s, ctx := newCycleStoreMV(t)
	tsk := mustCreateTaskMV(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{
		TaskID:      tsk.ID,
		TriggeredBy: domain.ActorAgent,
		Meta:        []byte(`{"runner":"cursor-cli"}`),
	})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if string(c.MetaJSON) != `{"runner":"cursor-cli"}` {
		t.Fatalf("meta_json = %q, want {\"runner\":\"cursor-cli\"}", string(c.MetaJSON))
	}
}

// TestStore_CompletePhase_details_normalizes_and_validates pins the same
// invariant for task_cycle_phases.details_json: null normalizes to "{}",
// non-object JSON is rejected, and an object passes through.
func TestStore_CompletePhase_details_normalizes_and_validates(t *testing.T) {
	s, ctx := newCycleStoreMV(t)
	tsk := mustCreateTaskMV(t, s, ctx)
	cycle, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	phase, err := s.StartPhase(ctx, cycle.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start phase: %v", err)
	}

	// null details normalize to "{}".
	out, err := s.CompletePhase(ctx, CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: phase.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		Details:  []byte("null"),
		By:       domain.ActorAgent,
	})
	if err != nil {
		t.Fatalf("complete phase with null details: %v", err)
	}
	if string(out.DetailsJSON) != "{}" {
		t.Fatalf("details_json = %q, want {}", string(out.DetailsJSON))
	}

	// Start a fresh cycle so we can run another phase end-to-end.
	if _, err := s.TerminateCycle(ctx, cycle.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
		t.Fatalf("terminate cycle: %v", err)
	}
	cycle2, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle 2: %v", err)
	}
	phase2, err := s.StartPhase(ctx, cycle2.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start phase 2: %v", err)
	}
	_, err = s.CompletePhase(ctx, CompletePhaseInput{
		CycleID:  cycle2.ID,
		PhaseSeq: phase2.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		Details:  []byte(`[1,2]`),
		By:       domain.ActorAgent,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("complete phase with array details err = %v, want ErrInvalidInput", err)
	}
}

func newCycleStoreMV(t *testing.T) (*Store, context.Context) {
	t.Helper()
	return NewStore(tasktestdb.OpenSQLite(t)), context.Background()
}

func mustCreateTaskMV(t *testing.T, s *Store, ctx context.Context) *domain.Task {
	t.Helper()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return tsk
}
