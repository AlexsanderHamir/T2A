package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// count_pre_feature.go owns the one-shot startup counter the agent
// worker supervisor logs on boot so operators can see how many cycles
// pre-date the per-task runner/model attribution V2 keys
// (cursor_model_effective). Call once at startup, log the result, and
// move on — there is no live re-count.
//
// Why a Go-side scan rather than `meta_json::jsonb -> ?` SQL: the
// substrate runs on Postgres in production AND SQLite in tests, and
// the JSON1 / jsonb operators differ. Decoding in Go matches the
// scan_runners.go pattern and keeps the helper portable; the cost is
// O(N) over terminal cycle rows on a single startup query that is
// already bounded by `agentWorkerStartupSweepTimeout`.

// preFeatureMetaProjection is the minimal subset of cycle meta we
// need to decide whether a row pre-dates the V2 attribution keys.
// We treat a row as pre-feature when cursor_model_effective is
// missing OR explicitly empty (D2: empty string is the truthful
// "no model configured" value, but for the rollout count we want
// the operator-friendly "how many rows still need attention?"
// answer, which lumps both into the same bucket).
type preFeatureMetaProjection struct {
	CursorModelEffective *string `json:"cursor_model_effective"`
}

// CountPreFeatureCycles returns the number of terminal task_cycles
// whose meta_json predates the V2 runner/model attribution keys
// (cursor_model_effective is missing). The empty-string case is
// counted separately so operators can tell "pre-feature row" from
// "feature ran but no model was configured" without re-querying.
//
// Cheap one-shot intended for supervisor startup; runs ONE SELECT
// over task_cycles and decodes meta in Go for portability with
// SQLite (used in tests). Safe to call on an empty database (returns
// 0/0/nil).
func CountPreFeatureCycles(ctx context.Context, db *gorm.DB) (PreFeatureCycleCounts, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.CountPreFeatureCycles")
	var rows []struct {
		Meta datatypes.JSON `gorm:"column:meta_json"`
	}
	if err := db.WithContext(ctx).Model(&domain.TaskCycle{}).
		Select("meta_json").
		Where("ended_at IS NOT NULL").
		Scan(&rows).Error; err != nil {
		return PreFeatureCycleCounts{}, fmt.Errorf("count pre-feature cycles: %w", err)
	}
	out := PreFeatureCycleCounts{Total: int64(len(rows))}
	for _, r := range rows {
		if len(r.Meta) == 0 {
			out.MissingKey++
			continue
		}
		var p preFeatureMetaProjection
		if err := json.Unmarshal(r.Meta, &p); err != nil {
			// A row with an undecodable meta_json is older than
			// V2 by construction (V2 always writes a clean
			// JSON object). Bucket it as "missing" rather than
			// dropping it.
			out.MissingKey++
			continue
		}
		if p.CursorModelEffective == nil {
			out.MissingKey++
			continue
		}
		if *p.CursorModelEffective == "" {
			out.EmptyValue++
		}
	}
	return out, nil
}

// PreFeatureCycleCounts breaks the rollout count into the two
// operator-meaningful buckets so the startup log line can render
// both without losing information.
type PreFeatureCycleCounts struct {
	// Total is the count of all terminal cycles inspected.
	Total int64
	// MissingKey is the count of terminal cycles whose meta_json
	// either has no `cursor_model_effective` key or whose meta is
	// otherwise undecodable. These are pre-V2 rows; their runner
	// and model attribution cannot be recovered without a one-shot
	// rewrite script.
	MissingKey int64
	// EmptyValue is the count of terminal cycles whose meta_json
	// carries `cursor_model_effective: ""`. These ran the V2 code
	// path but the resolved model was empty — typically because
	// neither tasks.cursor_model nor app_settings.cursor_model
	// was set at the time. Recoverable by configuring a default
	// model going forward, not by backfill.
	EmptyValue int64
}
