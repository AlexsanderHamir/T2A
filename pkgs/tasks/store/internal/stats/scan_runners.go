package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
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
type runnerStatsRow struct {
	Status    domain.CycleStatus
	StartedAt time.Time
	EndedAt   *time.Time
	Meta      datatypes.JSON `gorm:"column:meta_json"`
}

// runnerStatsMetaProjection mirrors the keys buildCycleMeta
// (pkgs/agents/worker/meta.go) writes for V2. Decoded per row; missing
// keys decode to "" which the bucketing code maps to the unknown /
// default bucket per its semantic rules.
type runnerStatsMetaProjection struct {
	Runner               string `json:"runner"`
	CursorModelEffective string `json:"cursor_model_effective"`
}

// scanRunnerStats reads every terminal cycle row, decodes the meta
// projection in Go, and assembles the three breakdown maps with
// succeeded-only p50/p95 percentiles per bucket. One SQL round-trip;
// O(N) memory in terminal cycle count (same scale as the existing
// scanCyclesByStatus query).
func scanRunnerStats(ctx context.Context, db *gorm.DB) (RunnerStats, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanRunnerStats")
	out := RunnerStats{
		ByRunner:      map[string]RunnerBucket{},
		ByModel:       map[string]RunnerBucket{},
		ByRunnerModel: map[string]RunnerBucket{},
	}
	var rows []runnerStatsRow
	if err := db.WithContext(ctx).Model(&domain.TaskCycle{}).
		Select("status, started_at, ended_at, meta_json").
		Where("ended_at IS NOT NULL").
		Scan(&rows).Error; err != nil {
		return RunnerStats{}, fmt.Errorf("runner stats: %w", err)
	}

	// Per-bucket counters and duration samples. Populated in one
	// pass over rows, then folded into out.* after percentiles
	// are computed. bucketAcc is declared at package scope so
	// bucketFromAcc can reference it.
	newBucket := func() *bucketAcc {
		return &bucketAcc{byStatus: map[domain.CycleStatus]int64{}}
	}
	runnerAcc := map[string]*bucketAcc{}
	modelAcc := map[string]*bucketAcc{}
	pairAcc := map[string]*bucketAcc{}

	for _, r := range rows {
		runner := RunnerUnknownKey
		model := ""
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
		pair := runner + "|" + model

		ra := runnerAcc[runner]
		if ra == nil {
			ra = newBucket()
			runnerAcc[runner] = ra
		}
		ma := modelAcc[model]
		if ma == nil {
			ma = newBucket()
			modelAcc[model] = ma
		}
		pa := pairAcc[pair]
		if pa == nil {
			pa = newBucket()
			pairAcc[pair] = pa
		}
		ra.byStatus[r.Status]++
		ma.byStatus[r.Status]++
		pa.byStatus[r.Status]++
		if r.Status == domain.CycleStatusSucceeded && r.EndedAt != nil {
			d := r.EndedAt.Sub(r.StartedAt).Seconds()
			if d < 0 {
				// clock skew — treat as zero rather than
				// dropping the row (the count still
				// matters for the by-status cell).
				d = 0
			}
			ra.succeededDur = append(ra.succeededDur, d)
			ma.succeededDur = append(ma.succeededDur, d)
			pa.succeededDur = append(pa.succeededDur, d)
		}
	}

	for k, b := range runnerAcc {
		out.ByRunner[k] = bucketFromAcc(b)
	}
	for k, b := range modelAcc {
		out.ByModel[k] = bucketFromAcc(b)
	}
	for k, b := range pairAcc {
		out.ByRunnerModel[k] = bucketFromAcc(b)
	}
	return out, nil
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
