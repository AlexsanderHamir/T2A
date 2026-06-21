package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// scan_runners.go owns the runner / model breakdown aggregation that
// backs the new `runner` block on GET /tasks/stats (Phase 2 of the
// per-task runner/model attribution plan). The scanner reads
// task_cycles.meta_json verbatim and aggregates in Go so the query
// stays portable across Postgres and SQLite — same pattern as
// scan_failures.go's data_json projection.
//
// Cardinality cap: NONE. The plan locked decision D7 ("no cap") so the
// audit trail is complete at any scale; if a deployment grows past a
// few hundred (runner, model) pairs the panel can adopt virtualization
// without a backend change.

// RunnerStats aggregates terminal cycles by adapter identity, by
// concrete model identifier, and by the (runner, model) pair. Every
// map is non-nil ({} on empty database) so the wire shape stays
// stable. Duration percentiles are SUCCEEDED-ONLY (decision D3) so
// failed runs that abort early do not skew the success-path latency
// the operator actually cares about.
type RunnerStats struct {
	// ByRunner aggregates terminal cycles by Runner.Name() (verbatim
	// from cycle_meta.runner). Cycles whose meta predates the V2
	// keys, or whose runner key is empty, fall into the bucket
	// keyed by RunnerUnknownKey so they remain countable.
	ByRunner map[string]RunnerBucket
	// ByModel aggregates terminal cycles by the runner's resolved
	// effective model (verbatim from cycle_meta.cursor_model_effective).
	// The empty-string key is preserved (NOT renamed to "default")
	// so the SPA can render the explicit "default model" bucket
	// without an extra projection. Pre-feature cycles also fall
	// here.
	ByModel map[string]RunnerBucket
	// ByRunnerModel keys the (runner|model) pair using a
	// pipe-delimited composite key. The frontend splits on the
	// delimiter to render the two-level table; pipe is used because
	// neither runner names nor model names contain "|" today.
	ByRunnerModel map[string]RunnerBucket
	// ByRunnerModelResolved keys the (runner|effective|resolved)
	// triple using a pipe-delimited composite key. Only populated
	// for cycles whose execute-phase details_json surfaced a non-
	// empty resolved_model (the cursor adapter lifts this from
	// cursor-agent's stream-json `system.init.model` event — the
	// only signal that exposes what model `auto` actually routed
	// to). Cycles without a resolved model are intentionally absent
	// from this map so the SPA can render "Cursor CLI · Auto →
	// Claude 4 Sonnet" style sub-rows only when there is a real
	// observation, not a placeholder.
	ByRunnerModelResolved map[string]RunnerBucket
}

// RunnerBucket is the per-bucket payload: the by-status counter the
// observability page already renders for the global block, plus the
// succeeded-only duration percentiles. Counts are non-nil; duration
// values are zero when there are no SUCCEEDED cycles in the bucket.
type RunnerBucket struct {
	ByStatus map[domain.CycleStatus]int64
	// Succeeded carries the raw success count (mirrors
	// ByStatus[CycleStatusSucceeded]) for caller convenience and
	// to avoid a "missing key" check on the percentile gate.
	Succeeded int64
	// DurationP50SucceededSeconds / DurationP95SucceededSeconds
	// are computed only over CycleStatusSucceeded rows (decision
	// D3). Both are 0 when Succeeded == 0; doc-comment pins this
	// so the SPA can decide whether to render "—" instead of
	// "0.00s" for empty buckets.
	DurationP50SucceededSeconds float64
	DurationP95SucceededSeconds float64
}

// RunnerUnknownKey is the bucket key used for cycles whose meta
// predates the V2 attribution keys (or whose runner is otherwise
// empty). Exported so the contract / handler tests can reference
// it without re-typing the literal.
const RunnerUnknownKey = "unknown"

// runnerStatsRowSelect picks only the columns we need from
// task_cycles. terminated cycles only (ended_at NOT NULL); the
// running bucket would skew duration percentiles and the
// by-status counts already have a "running" cell pinned by the
// global block.
//
// ExecDetails is the execute-phase details_json (LEFT JOIN'd via
// task_cycle_phases.phase='execute'). Nullable because a cycle may
// have terminated before its first execute phase opened (e.g. a
// store-error during StartCycle). The cursor adapter stuffs the
// stream-json-derived resolved_model in there, so scanRunnerStats
// lifts the value out without needing a second round-trip to the
// worker or a separate table.
type runnerStatsRow struct {
	Status      domain.CycleStatus
	StartedAt   time.Time
	EndedAt     *time.Time
	Meta        datatypes.JSON `gorm:"column:meta_json"`
	ExecDetails datatypes.JSON `gorm:"column:exec_details_json"`
}

// runnerStatsMetaProjection mirrors the keys buildCycleMeta
// (pkgs/agents/harness/meta.go) writes for V2. Decoded per row; missing
// keys decode to "" which the bucketing code maps to the unknown /
// default bucket per its semantic rules.
type runnerStatsMetaProjection struct {
	Runner               string `json:"runner"`
	CursorModelEffective string `json:"cursor_model_effective"`
}

// runnerStatsExecDetailsProjection mirrors the keys the cursor adapter
// writes into the execute phase's details_json via buildDetails. Only
// resolved_model is consumed by stats today; the rest of the payload
// (session/request ids, usage, etc.) is scoped to the per-phase audit
// trail and is not meaningful at the aggregate level.
type runnerStatsExecDetailsProjection struct {
	ResolvedModel string `json:"resolved_model"`
}

// scanRunnerStats reads every terminal cycle row, decodes the meta
// projection in Go, and assembles the four breakdown maps with
// succeeded-only p50/p95 percentiles per bucket. One SQL round-trip;
// O(N) memory in terminal cycle count (same scale as the existing
// scanCyclesByStatus query).
//
// The query LEFT JOINs task_cycle_phases filtered to phase='execute'
// so the execute-phase details_json is available on the same row.
// That JSON is where the cursor adapter persists the resolved_model
// (lifted from cursor-agent's stream-json `system.init.model` event).
// LEFT so pre-feature cycles and cycles that never reached the
// execute phase still contribute to the first three maps; they
// simply don't populate ByRunnerModelResolved.
func scanRunnerStats(ctx context.Context, db *gorm.DB) (RunnerStats, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanRunnerStats")
	rows, err := queryRunnerStatsRows(ctx, db)
	if err != nil {
		return RunnerStats{}, err
	}
	acc := newRunnerStatsAccumulators()
	for _, r := range rows {
		acc.accumulateRunnerStatsRow(r)
	}
	return acc.foldRunnerStats(), nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func newEmptyRunnerStats() RunnerStats {
	return RunnerStats{
		ByRunner:              map[string]RunnerBucket{},
		ByModel:               map[string]RunnerBucket{},
		ByRunnerModel:         map[string]RunnerBucket{},
		ByRunnerModelResolved: map[string]RunnerBucket{},
	}
}

//funclogmeasure:skip category=hot-path reason="DB read helper; operation trace is emitted by scanRunnerStats chokepoint."
func queryRunnerStatsRows(ctx context.Context, db *gorm.DB) ([]runnerStatsRow, error) {
	var rows []runnerStatsRow
	if err := db.WithContext(ctx).Model(&domain.TaskCycle{}).
		Select(
			"task_cycles.status AS status, "+
				"task_cycles.started_at AS started_at, "+
				"task_cycles.ended_at AS ended_at, "+
				"task_cycles.meta_json AS meta_json, "+
				"p.details_json AS exec_details_json").
		Joins("LEFT JOIN task_cycle_phases p ON p.cycle_id = task_cycles.id AND p.phase = ?",
			domain.PhaseExecute).
		Where("task_cycles.ended_at IS NOT NULL").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("runner stats: %w", err)
	}
	return rows, nil
}

func decodeRunnerStatsAttribution(r runnerStatsRow) (runner, model, resolved string) {
	runner = RunnerUnknownKey
	model = ""
	if len(r.Meta) > 0 {
		var p runnerStatsMetaProjection
		if err := json.Unmarshal(r.Meta, &p); err != nil {
			slog.Debug("runner stats meta decode skipped",
				"cmd", logCmd,
				"operation", "tasks.store.stats.scanRunnerStats.decode_skip",
				"err", err)
		} else {
			if p.Runner != "" {
				runner = p.Runner
			}
			model = p.CursorModelEffective
		}
	}
	resolved = ""
	if len(r.ExecDetails) > 0 {
		var d runnerStatsExecDetailsProjection
		if err := json.Unmarshal(r.ExecDetails, &d); err != nil {
			slog.Debug("runner stats exec details decode skipped",
				"cmd", logCmd,
				"operation", "tasks.store.stats.scanRunnerStats.exec_details_decode_skip",
				"err", err)
		} else {
			resolved = d.ResolvedModel
		}
	}
	return runner, model, resolved
}

// runnerStatsAccumulators holds per-bucket counters and duration
// samples while scanRunnerStats walks terminal cycle rows.
type runnerStatsAccumulators struct {
	byRunner   map[string]*bucketAcc
	byModel    map[string]*bucketAcc
	byPair     map[string]*bucketAcc
	byResolved map[string]*bucketAcc
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func newRunnerStatsAccumulators() *runnerStatsAccumulators {
	return &runnerStatsAccumulators{
		byRunner:   map[string]*bucketAcc{},
		byModel:    map[string]*bucketAcc{},
		byPair:     map[string]*bucketAcc{},
		byResolved: map[string]*bucketAcc{},
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func newBucketAcc() *bucketAcc {
	return &bucketAcc{byStatus: map[domain.CycleStatus]int64{}}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func bucketAccForKey(m map[string]*bucketAcc, key string) *bucketAcc {
	b := m[key]
	if b == nil {
		b = newBucketAcc()
		m[key] = b
	}
	return b
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (a *runnerStatsAccumulators) accumulateRunnerStatsRow(r runnerStatsRow) {
	runner, model, resolved := decodeRunnerStatsAttribution(r)
	pair := runner + "|" + model

	ra := bucketAccForKey(a.byRunner, runner)
	ma := bucketAccForKey(a.byModel, model)
	pa := bucketAccForKey(a.byPair, pair)

	ra.byStatus[r.Status]++
	ma.byStatus[r.Status]++
	pa.byStatus[r.Status]++

	// Only record in the resolved-model breakdown when the adapter
	// actually observed one. Avoids polluting the panel with
	// "<unknown>" sub-rows for pre-feature cycles.
	var resolvedBucket *bucketAcc
	if resolved != "" {
		triple := runner + "|" + model + "|" + resolved
		resolvedBucket = bucketAccForKey(a.byResolved, triple)
		resolvedBucket.byStatus[r.Status]++
	}

	if r.Status == domain.CycleStatusSucceeded && r.EndedAt != nil {
		d := r.EndedAt.Sub(r.StartedAt).Seconds()
		if d < 0 {
			// clock skew — treat as zero rather than dropping the
			// row (the count still matters for the by-status cell).
			d = 0
		}
		ra.succeededDur = append(ra.succeededDur, d)
		ma.succeededDur = append(ma.succeededDur, d)
		pa.succeededDur = append(pa.succeededDur, d)
		if resolvedBucket != nil {
			resolvedBucket.succeededDur = append(resolvedBucket.succeededDur, d)
		}
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (a *runnerStatsAccumulators) foldRunnerStats() RunnerStats {
	out := newEmptyRunnerStats()
	for k, b := range a.byRunner {
		out.ByRunner[k] = bucketFromAcc(b)
	}
	for k, b := range a.byModel {
		out.ByModel[k] = bucketFromAcc(b)
	}
	for k, b := range a.byPair {
		out.ByRunnerModel[k] = bucketFromAcc(b)
	}
	for k, b := range a.byResolved {
		out.ByRunnerModelResolved[k] = bucketFromAcc(b)
	}
	return out
}

// bucketAcc is the per-bucket accumulator threaded through
// scanRunnerStats. Holds the per-status counters plus the raw
// succeeded-only duration samples we percentile-fold in
// bucketFromAcc.
type bucketAcc struct {
	byStatus     map[domain.CycleStatus]int64
	succeededDur []float64
}

// bucketFromAcc folds a per-bucket accumulator into the wire-facing
// RunnerBucket. percentile() is called twice per bucket; the input
// slice is sorted in place once at the top so the second call is
// O(1) on the already-sorted data.
func bucketFromAcc(b *bucketAcc) RunnerBucket {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.bucketFromAcc",
		"by_status_keys", len(b.byStatus), "succeeded_samples", len(b.succeededDur))
	out := RunnerBucket{
		ByStatus:  b.byStatus,
		Succeeded: b.byStatus[domain.CycleStatusSucceeded],
	}
	if len(b.succeededDur) > 0 {
		sort.Float64s(b.succeededDur)
		out.DurationP50SucceededSeconds = percentileSorted(b.succeededDur, 0.50)
		out.DurationP95SucceededSeconds = percentileSorted(b.succeededDur, 0.95)
	}
	return out
}

// percentileSorted returns the q-th percentile (0..1) of an
// already-sorted slice using the nearest-rank method (no
// interpolation): straightforward, deterministic, and matches the
// "p50/p95 of succeeded runs" mental model operators expect from
// dashboards. Empty slice returns 0.
func percentileSorted(sorted []float64, q float64) float64 {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.percentileSorted",
		"n", len(sorted), "q", q)
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[n-1]
	}
	// Nearest-rank: rank = ceil(q * n), 1-indexed.
	rank := int(math.Ceil(q * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}
