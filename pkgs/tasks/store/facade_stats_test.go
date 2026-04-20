package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// emptyRunnerStats is the Phase 2 invariant: every map in the
// Runner block must be non-nil ({} on the wire) so the SPA can
// iterate without nil-guards. Asserted in the empty-DB test below.
func assertEmptyRunnerStats(t *testing.T, got TaskStats) {
	t.Helper()
	if got.Runner.ByRunner == nil {
		t.Fatalf("Runner.ByRunner must be non-nil on empty DB")
	}
	if got.Runner.ByModel == nil {
		t.Fatalf("Runner.ByModel must be non-nil on empty DB")
	}
	if got.Runner.ByRunnerModel == nil {
		t.Fatalf("Runner.ByRunnerModel must be non-nil on empty DB")
	}
	if len(got.Runner.ByRunner) != 0 || len(got.Runner.ByModel) != 0 || len(got.Runner.ByRunnerModel) != 0 {
		t.Fatalf("Runner block non-empty on empty DB: %+v", got.Runner)
	}
}

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
	assertEmptyRunnerStats(t, got)
}

// TestStore_TaskStats_runnerBreakdown_aggregatesByRunnerModelAndPair
// drives three terminated cycles through StartCycle / TerminateCycle
// with different (runner, model) meta payloads and asserts the
// Runner block aggregates them correctly across all three breakdowns
// (by_runner / by_model / by_runner_model). This is the integration
// guard for the Phase 2 wire shape on /tasks/stats.
func TestStore_TaskStats_runnerBreakdown_aggregatesByRunnerModelAndPair(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	type seed struct {
		meta   string
		status domain.CycleStatus
	}
	seeds := []seed{
		{meta: `{"runner":"cursor-cli","cursor_model_effective":"sonnet-4.5"}`, status: domain.CycleStatusSucceeded},
		{meta: `{"runner":"cursor-cli","cursor_model_effective":"sonnet-4.5"}`, status: domain.CycleStatusFailed},
		{meta: `{"runner":"cursor-cli","cursor_model_effective":"opus-4"}`, status: domain.CycleStatusSucceeded},
	}
	for i, sd := range seeds {
		cyc, err := s.StartCycle(ctx, StartCycleInput{
			TaskID:      tsk.ID,
			TriggeredBy: domain.ActorAgent,
			Meta:        []byte(sd.meta),
		})
		if err != nil {
			t.Fatalf("seed %d StartCycle: %v", i, err)
		}
		if _, err := s.TerminateCycle(ctx, cyc.ID, sd.status, "seed", domain.ActorAgent); err != nil {
			t.Fatalf("seed %d TerminateCycle: %v", i, err)
		}
	}

	got, err := s.TaskStats(ctx)
	if err != nil {
		t.Fatalf("TaskStats: %v", err)
	}

	cursorBucket, ok := got.Runner.ByRunner["cursor-cli"]
	if !ok {
		t.Fatalf("Runner.ByRunner missing cursor-cli; got keys=%v", mapKeys(got.Runner.ByRunner))
	}
	if cursorBucket.ByStatus[domain.CycleStatusSucceeded] != 2 {
		t.Errorf("ByRunner[cursor-cli].succeeded=%d want 2",
			cursorBucket.ByStatus[domain.CycleStatusSucceeded])
	}
	if cursorBucket.ByStatus[domain.CycleStatusFailed] != 1 {
		t.Errorf("ByRunner[cursor-cli].failed=%d want 1",
			cursorBucket.ByStatus[domain.CycleStatusFailed])
	}
	if cursorBucket.Succeeded != 2 {
		t.Errorf("ByRunner[cursor-cli].Succeeded=%d want 2", cursorBucket.Succeeded)
	}

	sonnet, ok := got.Runner.ByModel["sonnet-4.5"]
	if !ok {
		t.Fatalf("Runner.ByModel missing sonnet-4.5; got keys=%v", mapKeys(got.Runner.ByModel))
	}
	if sonnet.ByStatus[domain.CycleStatusSucceeded] != 1 || sonnet.ByStatus[domain.CycleStatusFailed] != 1 {
		t.Errorf("ByModel[sonnet-4.5]=%+v want succeeded=1, failed=1", sonnet.ByStatus)
	}
	opus, ok := got.Runner.ByModel["opus-4"]
	if !ok {
		t.Fatalf("Runner.ByModel missing opus-4; got keys=%v", mapKeys(got.Runner.ByModel))
	}
	if opus.ByStatus[domain.CycleStatusSucceeded] != 1 {
		t.Errorf("ByModel[opus-4].succeeded=%d want 1", opus.ByStatus[domain.CycleStatusSucceeded])
	}

	pairKey := "cursor-cli|sonnet-4.5"
	pair, ok := got.Runner.ByRunnerModel[pairKey]
	if !ok {
		t.Fatalf("Runner.ByRunnerModel missing %q; got keys=%v",
			pairKey, mapKeys(got.Runner.ByRunnerModel))
	}
	if pair.ByStatus[domain.CycleStatusSucceeded] != 1 || pair.ByStatus[domain.CycleStatusFailed] != 1 {
		t.Errorf("ByRunnerModel[%q]=%+v want succeeded=1, failed=1", pairKey, pair.ByStatus)
	}
}

// TestStore_CountPreFeatureCycles_bucketsByMissingVsEmptyEffectiveModel
// pins the rollout-count contract used by the agent worker supervisor's
// startup log line. Three terminated cycles seed three buckets:
//   - V2 row with a real effective model: NEITHER MissingKey nor
//     EmptyValue increments (counted only in Total).
//   - V2 row with explicit empty effective model: EmptyValue
//     increments. This is the operator-friendly "feature ran but no
//     model configured" bucket.
//   - Pre-V2 row missing the key entirely: MissingKey increments.
//     This is the "needs a one-shot rewrite to recover" bucket.
//
// Running cycles MUST NOT count (`ended_at IS NOT NULL` filter); the
// log line is about historical, attributed runs, not in-flight work.
func TestStore_CountPreFeatureCycles_bucketsByMissingVsEmptyEffectiveModel(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	seeds := []struct {
		meta string
	}{
		{meta: `{"runner":"cursor-cli","cursor_model_effective":"opus-4"}`},
		{meta: `{"runner":"cursor-cli","cursor_model_effective":""}`},
		{meta: `{"runner":"cursor-cli"}`},
	}
	for i, sd := range seeds {
		cyc, err := s.StartCycle(ctx, StartCycleInput{
			TaskID:      tsk.ID,
			TriggeredBy: domain.ActorAgent,
			Meta:        []byte(sd.meta),
		})
		if err != nil {
			t.Fatalf("seed %d StartCycle: %v", i, err)
		}
		if _, err := s.TerminateCycle(ctx, cyc.ID,
			domain.CycleStatusSucceeded, "seed", domain.ActorAgent); err != nil {
			t.Fatalf("seed %d TerminateCycle: %v", i, err)
		}
	}
	// One running cycle to prove the ended_at filter excludes it.
	if _, err := s.StartCycle(ctx, StartCycleInput{
		TaskID: tsk.ID, TriggeredBy: domain.ActorAgent,
		Meta: []byte(`{}`),
	}); err != nil {
		t.Fatalf("running-cycle seed StartCycle: %v", err)
	}

	got, err := s.CountPreFeatureCycles(ctx)
	if err != nil {
		t.Fatalf("CountPreFeatureCycles: %v", err)
	}
	if got.Total != 3 {
		t.Errorf("Total=%d want 3 (running cycle must be excluded)", got.Total)
	}
	if got.MissingKey != 1 {
		t.Errorf("MissingKey=%d want 1", got.MissingKey)
	}
	if got.EmptyValue != 1 {
		t.Errorf("EmptyValue=%d want 1", got.EmptyValue)
	}
}

// TestStore_CountPreFeatureCycles_emptyDatabase pins the boot-on-fresh-DB
// behaviour: zero rows, no error. The supervisor relies on this so a
// brand-new deployment logs "0 / 0 / 0" rather than failing startup.
func TestStore_CountPreFeatureCycles_emptyDatabase(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	got, err := s.CountPreFeatureCycles(context.Background())
	if err != nil {
		t.Fatalf("CountPreFeatureCycles: %v", err)
	}
	if got.Total != 0 || got.MissingKey != 0 || got.EmptyValue != 0 {
		t.Errorf("empty DB counts non-zero: %+v", got)
	}
}

func mapKeys(m map[string]RunnerBucket) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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
