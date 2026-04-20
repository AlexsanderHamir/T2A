package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// TestStore_TaskStats_emptyDatabase pins the store-side invariant that
// every map in TaskStats is non-nil and every domain.Phase enum key is
// present in Phases.ByPhaseStatus on a fresh database. The handler's
// HTTP contract test relies on this guarantee.
func TestStore_TaskStats_emptyDatabase(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	got, err := s.TaskStats(context.Background())
	if err != nil {
		t.Fatalf("TaskStats: %v", err)
	}
	if got.ByStatus == nil || got.ByPriority == nil || got.ByScope == nil {
		t.Fatalf("totals maps must be non-nil on empty DB: %+v", got)
	}
	if got.Cycles.ByStatus == nil || got.Cycles.ByTriggeredBy == nil {
		t.Fatalf("cycles maps must be non-nil on empty DB: %+v", got.Cycles)
	}
	if got.Phases.ByPhaseStatus == nil {
		t.Fatalf("phases.by_phase_status must be non-nil on empty DB")
	}
	wantPhases := []domain.Phase{
		domain.PhaseDiagnose, domain.PhaseExecute,
		domain.PhaseVerify, domain.PhasePersist,
	}
	for _, p := range wantPhases {
		inner, ok := got.Phases.ByPhaseStatus[p]
		if !ok {
			t.Errorf("phases.by_phase_status[%q] missing on empty DB", p)
			continue
		}
		if inner == nil {
			t.Errorf("phases.by_phase_status[%q] inner map is nil; want {}", p)
		}
		if len(inner) != 0 {
			t.Errorf("phases.by_phase_status[%q]=%v want {}", p, inner)
		}
	}
	if got.RecentFailures == nil {
		t.Fatalf("RecentFailures must be non-nil on empty DB (use empty slice, not nil)")
	}
	if len(got.RecentFailures) != 0 {
		t.Fatalf("RecentFailures=%v want [] on empty DB", got.RecentFailures)
	}
}

// TestStore_TaskStats_populatesCyclesPhasesAndFailures drives one cycle
// through diagnose/execute and terminates it as failed, then asserts
// every aggregation in the cycles/phases/recent_failures blocks
// reflects the data. This is the integration check that lets us trust
// the Observability page numbers.
func TestStore_TaskStats_populatesCyclesPhasesAndFailures(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("StartCycle: %v", err)
	}
	dx, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("StartPhase diagnose: %v", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{
		CycleID: cyc.ID, PhaseSeq: dx.PhaseSeq,
		Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("CompletePhase diagnose: %v", err)
	}
	ex, err := s.StartPhase(ctx, cyc.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("StartPhase execute: %v", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{
		CycleID: cyc.ID, PhaseSeq: ex.PhaseSeq,
		Status: domain.PhaseStatusFailed, Summary: ptrString("execute blew up"),
		By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("CompletePhase execute (failed): %v", err)
	}
	if _, err := s.TerminateCycle(ctx, cyc.ID,
		domain.CycleStatusFailed, "execute blew up", domain.ActorAgent); err != nil {
		t.Fatalf("TerminateCycle: %v", err)
	}

	got, err := s.TaskStats(ctx)
	if err != nil {
		t.Fatalf("TaskStats: %v", err)
	}

	if got.Cycles.ByStatus[domain.CycleStatusFailed] != 1 {
		t.Fatalf("cycles.by_status[failed]=%d want 1: %+v",
			got.Cycles.ByStatus[domain.CycleStatusFailed], got.Cycles.ByStatus)
	}
	if got.Cycles.ByTriggeredBy[domain.ActorAgent] != 1 {
		t.Fatalf("cycles.by_triggered_by[agent]=%d want 1: %+v",
			got.Cycles.ByTriggeredBy[domain.ActorAgent], got.Cycles.ByTriggeredBy)
	}
	if got.Phases.ByPhaseStatus[domain.PhaseDiagnose][domain.PhaseStatusSucceeded] != 1 {
		t.Fatalf("phases.diagnose.succeeded=%d want 1",
			got.Phases.ByPhaseStatus[domain.PhaseDiagnose][domain.PhaseStatusSucceeded])
	}
	if got.Phases.ByPhaseStatus[domain.PhaseExecute][domain.PhaseStatusFailed] != 1 {
		t.Fatalf("phases.execute.failed=%d want 1",
			got.Phases.ByPhaseStatus[domain.PhaseExecute][domain.PhaseStatusFailed])
	}
	if len(got.RecentFailures) != 1 {
		t.Fatalf("len(RecentFailures)=%d want 1: %+v", len(got.RecentFailures), got.RecentFailures)
	}
	rf := got.RecentFailures[0]
	if rf.TaskID != tsk.ID {
		t.Errorf("RecentFailures[0].TaskID=%q want %q", rf.TaskID, tsk.ID)
	}
	if rf.CycleID != cyc.ID {
		t.Errorf("RecentFailures[0].CycleID=%q want %q", rf.CycleID, cyc.ID)
	}
	if rf.AttemptSeq != cyc.AttemptSeq {
		t.Errorf("RecentFailures[0].AttemptSeq=%d want %d", rf.AttemptSeq, cyc.AttemptSeq)
	}
	if rf.Status != string(domain.CycleStatusFailed) {
		t.Errorf("RecentFailures[0].Status=%q want %q", rf.Status, domain.CycleStatusFailed)
	}
	if rf.Reason != "execute blew up" {
		t.Errorf("RecentFailures[0].Reason=%q want %q", rf.Reason, "execute blew up")
	}
	if rf.EventSeq <= 0 {
		t.Errorf("RecentFailures[0].EventSeq=%d want >0", rf.EventSeq)
	}
}
