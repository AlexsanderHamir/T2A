package kernel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NextEventSeq returns the next monotonic seq for taskID inside the open
// transaction tx. Used by every audit-emitting path (CRUD, checklist,
// cycles, phases, devmirror, public AppendTaskEvent).
func NextEventSeq(tx *gorm.DB, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.NextEventSeq")
	var max int64
	err := tx.Raw(`SELECT COALESCE(MAX(seq), 0) FROM task_events WHERE task_id = ?`, taskID).Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("next event seq: %w", err)
	}
	return max + 1, nil
}

// AppendEvent inserts one task_events row inside the open transaction tx.
// data may be nil; callers wanting an empty JSON object should pass nil and
// the helper substitutes "{}" so downstream consumers never see SQL NULL.
func AppendEvent(tx *gorm.DB, taskID string, seq int64, typ domain.EventType, by domain.Actor, data []byte) error {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.AppendEvent")
	if data == nil {
		data = []byte("{}")
	}
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
