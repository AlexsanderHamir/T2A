package kernel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NextEventSeq returns the next monotonic seq for taskID inside the open
// transaction tx. Used by every audit-emitting path (CRUD, checklist,
// cycles, phases, devmirror, public AppendTaskEvent).
//
// Concurrency: two transactions racing to append events for the same
// task previously both read MAX(seq) = N and both tried to insert at
// seq = N+1, hitting `task_events_pkey` (composite PK on
// (task_id, seq)) with SQLSTATE 23505 — observed in production from
// parallel POST /tasks/{id}/checklist/items requests fired by the
// create-task modal. We serialize writers per task by row-locking the
// parent `tasks` row (`SELECT ... FOR UPDATE`) before reading
// MAX(seq); the lock is held for the rest of the caller's
// transaction, so the AppendEvent that follows is guaranteed to be
// the only writer at this seq. Lock granularity is the single task
// row (same chokepoint already used by tasks/crud.Update and
// events/thread.MarkResponded), so concurrent appends to *different*
// tasks remain fully parallel.
//
// SQLite is excluded — it serializes all writers globally, so
// `FOR UPDATE` is unnecessary (and unsupported pre-3.45). Mirrors the
// dialect guard in events/thread.MarkResponded.
func NextEventSeq(tx *gorm.DB, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.NextEventSeq")
	if tx.Dialector.Name() != "sqlite" {
		var locked domain.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Where("id = ?", taskID).First(&locked).Error; err != nil {
			return 0, fmt.Errorf("lock task for event seq: %w", err)
		}
	}
	var max int64
	err := tx.Raw(`SELECT COALESCE(MAX(seq), 0) FROM task_events WHERE task_id = ?`, taskID).Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("next event seq: %w", err)
	}
	return max + 1, nil
}

// AppendEvent inserts one task_events row inside the open transaction tx.
//
// data is normalized through NormalizeJSONObject so the on-disk shape of
// task_events.data_json honours the documented "always a JSON object"
// invariant (see docs/API-HTTP.md GET /tasks/{id}/events). nil, empty,
// whitespace-only, or the literal "null" all collapse to "{}" so downstream
// consumers (handler readers, SSE fan-out, /events keyset paging) never
// observe SQL NULL or a JSON null literal even if a future caller forgets
// the chokepoint at its own boundary. Non-object payloads (string / number
// / array / bool / malformed) surface as domain.ErrInvalidInput so the bug
// is caught at the writing call site instead of leaking past the read-side
// normalizeJSONObjectForResponse defense.
func AppendEvent(tx *gorm.DB, taskID string, seq int64, typ domain.EventType, by domain.Actor, data []byte) error {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.AppendEvent")
	normalized, err := NormalizeJSONObject(data, "data")
	if err != nil {
		return err
	}
	data = normalized
	ev := domain.TaskEvent{
		TaskID: taskID,
		Seq:    seq,
		At:     time.Now().UTC(),
		Type:   typ,
		By:     by,
		Data:   datatypes.JSON(data),
	}
	if err := tx.Omit("Task").Create(&ev).Error; err != nil {
		return fmt.Errorf("insert task_event: %w", err)
	}
	return nil
}

// EventPairJSON marshals a {"from": from, "to": to} payload used by
// status / priority / type transition audit events.
func EventPairJSON(from, to string) ([]byte, error) {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.EventPairJSON")
	b, err := json.Marshal(map[string]string{"from": from, "to": to})
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	return b, nil
}
