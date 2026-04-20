// Package stats owns the global task-counters query that backs GET
// /tasks/stats. The public store facade re-exports TaskStats and the
// Get function via (*Store).TaskStats. The shape (Total / Ready /
// Critical / ByStatus / ByPriority / ByScope) is the HTTP response
// contract — see handler_http_list_stats_contract_test.go.
package stats

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"gorm.io/gorm"
)

const logCmd = "taskapi"

// TaskStats holds the global task counters. Tests pin the invariant
// that every map field is non-nil (empty `{}` on empty database, never
// `null`) and that RecentFailures is non-nil (empty slice). The HTTP
// handler relies on this invariant to serve a stable wire shape.
type TaskStats struct {
	Total          int64
	Ready          int64
	Critical       int64
	ByStatus       map[domain.Status]int64
	ByPriority     map[domain.Priority]int64
	ByScope        map[string]int64
	Cycles         CycleStats
	Phases         PhaseStats
	RecentFailures []RecentFailure
}

// CycleStats aggregates task_cycles for the Observability page. Both
// maps are always non-nil; absent enum keys mean zero.
type CycleStats struct {
	ByStatus      map[domain.CycleStatus]int64
	ByTriggeredBy map[domain.Actor]int64
}

// PhaseStats aggregates task_cycle_phases by (phase, status) — the
// "failed in failed stage" matrix the Observability page renders as a
// heatmap. ByPhaseStatus[phase] is always present for every domain
// Phase value; the inner map is non-nil but only carries enum keys with
// nonzero count.
type PhaseStats struct {
	ByPhaseStatus map[domain.Phase]map[domain.PhaseStatus]int64
}

// allPhases is the canonical Phase list seeded into PhaseStats so the
// outer map always carries every enum key — empty inner map for phases
// that have never run, populated for those that have.
var allPhases = []domain.Phase{
	domain.PhaseDiagnose,
	domain.PhaseExecute,
	domain.PhaseVerify,
	domain.PhasePersist,
}

// Get returns global counters across all tasks. Six SQL round-trips:
// totals, by-status, by-priority, cycles-by-status,
// cycles-by-triggered-by, phases-by-(phase,status), and one more for
// recent cycle_failed mirror events. The Cycles / Phases /
// RecentFailures blocks are always populated (with empty maps / slices
// on a fresh database) so the HTTP wire shape stays stable — pinned by
// handler_http_list_stats_contract_test.go.
func Get(ctx context.Context, db *gorm.DB) (TaskStats, error) {
	defer kernel.DeferLatency(kernel.OpTaskStats)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.Get")
	r, err := scanTotals(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	out := TaskStats{
		Total:      r.Total,
		Ready:      r.Ready,
		Critical:   r.Critical,
		ByStatus:   map[domain.Status]int64{},
		ByPriority: map[domain.Priority]int64{},
		ByScope: map[string]int64{
			"parent":  r.ParentTotal,
			"subtask": r.SubtaskTotal,
		},
		Cycles: CycleStats{
			ByStatus:      map[domain.CycleStatus]int64{},
			ByTriggeredBy: map[domain.Actor]int64{},
		},
		Phases: PhaseStats{
			ByPhaseStatus: make(map[domain.Phase]map[domain.PhaseStatus]int64, len(allPhases)),
		},
		RecentFailures: []RecentFailure{},
	}
	for _, p := range allPhases {
		out.Phases.ByPhaseStatus[p] = map[domain.PhaseStatus]int64{}
	}
	statusRows, err := scanByStatus(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, sr := range statusRows {
		out.ByStatus[sr.Status] = sr.Count
	}
	priorityRows, err := scanByPriority(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, pr := range priorityRows {
		out.ByPriority[pr.Priority] = pr.Count
	}
	cycleStatusRows, err := scanCyclesByStatus(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, c := range cycleStatusRows {
		out.Cycles.ByStatus[c.Status] = c.Count
	}
	cycleActorRows, err := scanCyclesByTriggeredBy(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, c := range cycleActorRows {
		out.Cycles.ByTriggeredBy[c.TriggeredBy] = c.Count
	}
	phaseRows, err := scanPhasesByStatus(ctx, db)
	if err != nil {
		return TaskStats{}, err
	}
	for _, p := range phaseRows {
		bucket, ok := out.Phases.ByPhaseStatus[p.Phase]
		if !ok {
			// Unknown enum (forward-compat): seed lazily so the
			// query result is never silently dropped.
			bucket = map[domain.PhaseStatus]int64{}
			out.Phases.ByPhaseStatus[p.Phase] = bucket
		}
		bucket[p.Status] = p.Count
	}
	failures, err := scanRecentFailures(ctx, db, RecentFailureLimit)
	if err != nil {
		return TaskStats{}, err
	}
	out.RecentFailures = failures
	return out, nil
}
