package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RecentFailureLimit caps the recent_failures slice on the wire so the
// /tasks/stats payload stays bounded under load. Picked to render in a
// single scrollable card on the Observability page; raise carefully —
// the frontend table assumes a small N (no virtualization).
const RecentFailureLimit = 25

// RecentFailure is one row in the recent_failures slice on /tasks/stats.
// Fields are the projection the Observability page renders directly:
// task id (deep link), event seq (deep link), wall-clock instant, the
// cycle's attempt_seq, terminal status (failed|aborted), and a short
// human-readable reason when one was recorded. Keep this struct narrow:
// every new column widens the wire envelope and the contract test.
type RecentFailure struct {
	TaskID     string
	EventSeq   int64
	At         time.Time
	CycleID    string
	AttemptSeq int64
	// Status is the terminal CycleStatus the worker recorded ("failed"
	// or "aborted"). EventCycleFailed folds both cases per
	// mirrorEventTypeForCycleStatus, so we recover the distinction
	// from the event payload.
	Status string
	Reason string
}

// cycleFailedRow is the raw projection from task_events for one
// cycle_failed mirror; we unmarshal data_json in Go (rather than
// jsonb-extract in SQL) so the scanner stays portable across Postgres
// and SQLite.
type cycleFailedRow struct {
	TaskID string
	Seq    int64
	At     time.Time
	Data   datatypes.JSON `gorm:"column:data_json"`
}

// cycleFailedPayload mirrors the keys terminatedPayload writes for
// EventCycleFailed in pkgs/tasks/store/internal/cycles/cycles.go. Keys
// kept in sync with that producer; the dual-write invariant test would
// catch a divergence on the producer side.
type cycleFailedPayload struct {
	CycleID    string `json:"cycle_id"`
	AttemptSeq int64  `json:"attempt_seq"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

// scanRecentFailures returns the last `limit` cycle_failed mirror rows
// ordered by event timestamp descending (newest first). Rows whose
// data_json is malformed or missing required fields are skipped (logged
// at Debug) rather than failing the whole query — a lossy but operator-
// friendly behavior consistent with the rest of the timeline reads.
func scanRecentFailures(ctx context.Context, db *gorm.DB, limit int) ([]RecentFailure, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.stats.scanRecentFailures",
		"limit", limit)
	if limit <= 0 || limit > RecentFailureLimit {
		limit = RecentFailureLimit
	}
	var rows []cycleFailedRow
	if err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Select("task_id, seq, at, data_json").
		Where("type = ?", string(domain.EventCycleFailed)).
		Order("at DESC, seq DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("recent failures: %w", err)
	}
	out := make([]RecentFailure, 0, len(rows))
	for _, r := range rows {
		var p cycleFailedPayload
		if err := json.Unmarshal(r.Data, &p); err != nil {
			slog.Debug("recent failure decode skipped",
				"cmd", logCmd,
				"operation", "tasks.store.stats.scanRecentFailures.decode_skip",
				"task_id", r.TaskID, "seq", r.Seq, "err", err)
			continue
		}
		out = append(out, RecentFailure{
			TaskID:     r.TaskID,
			EventSeq:   r.Seq,
			At:         r.At,
			CycleID:    p.CycleID,
			AttemptSeq: p.AttemptSeq,
			Status:     p.Status,
			Reason:     p.Reason,
		})
	}
	return out, nil
}
