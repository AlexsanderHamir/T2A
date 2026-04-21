package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

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
//
// Reason starts as the cycle_failed mirror payload (often
// runner_non_zero_exit) and is then replaced when a matching
// phase_failed audit event carries a clearer summary or
// details.standardized_message so Observability matches the task
// timeline.
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
	out := decodeCycleFailedRows(rows)
	enrichRecentFailuresFromPhaseEvents(ctx, db, out)
	return out, nil
}

// decodeCycleFailedRows maps raw cycle_failed mirror rows to RecentFailure
// values. Malformed data_json rows are skipped (Debug log) so callers
// stay lossy-but-resilient.
func decodeCycleFailedRows(rows []cycleFailedRow) []RecentFailure {
	out := make([]RecentFailure, 0, len(rows))
	for _, r := range rows {
		var p cycleFailedPayload
		if err := json.Unmarshal(r.Data, &p); err != nil {
			slog.Debug("recent failure decode skipped",
				"cmd", logCmd,
				"operation", "tasks.store.stats.decodeCycleFailedRows",
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
	return out
}

// phaseFailedMirrorPayload is the subset of phaseTerminatedPayload
// (pkgs/tasks/store/internal/cycles) needed to surface operator-facing
// failure text on the Observability page.
type phaseFailedMirrorPayload struct {
	CycleID string         `json:"cycle_id"`
	Summary string         `json:"summary,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

const maxObservabilityReasonRunes = 800

// enrichRecentFailuresFromPhaseEvents overlays each row's Reason with
// text from the matching phase_failed mirror (same task_id + cycle_id)
// when present, so the stats table shows the same substance as the
// task timeline (e.g. cursor usage limit) instead of only
// runner_non_zero_exit.
func enrichRecentFailuresFromPhaseEvents(ctx context.Context, db *gorm.DB, failures []RecentFailure) {
	if len(failures) == 0 {
		return
	}
	needed := make(map[string]struct{}, len(failures))
	taskSeen := make(map[string]struct{})
	var taskIDs []string
	for _, f := range failures {
		needed[f.TaskID+"\x00"+f.CycleID] = struct{}{}
		if _, ok := taskSeen[f.TaskID]; !ok {
			taskSeen[f.TaskID] = struct{}{}
			taskIDs = append(taskIDs, f.TaskID)
		}
	}
	type row struct {
		TaskID string
		Seq    int64
		Data   datatypes.JSON `gorm:"column:data_json"`
	}
	var rows []row
	if err := db.WithContext(ctx).Model(&domain.TaskEvent{}).
		Select("task_id, seq, data_json").
		Where("type = ?", string(domain.EventPhaseFailed)).
		Where("task_id IN ?", taskIDs).
		Order("seq DESC").
		Limit(5000).
		Scan(&rows).Error; err != nil {
		slog.Debug("recent failures phase enrich skipped", "cmd", logCmd,
			"operation", "tasks.store.stats.enrichRecentFailuresFromPhaseEvents",
			"err", err)
		return
	}
	phaseByKey := make(map[string]*phaseFailedMirrorPayload)
	for _, r := range rows {
		var p phaseFailedMirrorPayload
		if err := json.Unmarshal(r.Data, &p); err != nil || p.CycleID == "" {
			continue
		}
		k := r.TaskID + "\x00" + p.CycleID
		if _, want := needed[k]; !want {
			continue
		}
		if _, have := phaseByKey[k]; have {
			continue
		}
		pp := p
		phaseByKey[k] = &pp
		if len(phaseByKey) == len(needed) {
			break
		}
	}
	for i := range failures {
		k := failures[i].TaskID + "\x00" + failures[i].CycleID
		ph := phaseByKey[k]
		if r := observabilityReasonFromPhaseAndCycle(failures[i].Reason, ph); r != "" {
			failures[i].Reason = r
		}
	}
}

func observabilityReasonFromPhaseAndCycle(cycleReason string, phase *phaseFailedMirrorPayload) string {
	if phase == nil {
		return ""
	}
	if msg := standardizedMessageFromDetails(phase.Details); msg != "" {
		return truncateReasonRunes(msg, maxObservabilityReasonRunes)
	}
	if s := strings.TrimSpace(phase.Summary); s != "" {
		return truncateReasonRunes(s, maxObservabilityReasonRunes)
	}
	if fk := failureKindFromDetails(phase.Details); fk != "" {
		if h := humanizeFailureKind(fk); h != "" {
			return h
		}
		return fk
	}
	if strings.TrimSpace(cycleReason) == "" {
		return ""
	}
	return cycleReason
}

func standardizedMessageFromDetails(d map[string]any) string {
	if d == nil {
		return ""
	}
	v, ok := d["standardized_message"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func failureKindFromDetails(d map[string]any) string {
	if d == nil {
		return ""
	}
	v, ok := d["failure_kind"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func humanizeFailureKind(kind string) string {
	switch kind {
	case "cursor_usage_limit":
		return "Cursor usage limit reached"
	default:
		return ""
	}
}

func truncateReasonRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}
