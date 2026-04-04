package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func nextEventSeq(tx *gorm.DB, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.nextEventSeq")
	var max int64
	err := tx.Raw(`SELECT COALESCE(MAX(seq), 0) FROM task_events WHERE task_id = ?`, taskID).Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("next event seq: %w", err)
	}
	return max + 1, nil
}

func appendEvent(tx *gorm.DB, taskID string, seq int64, typ domain.EventType, by domain.Actor, data []byte) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.appendEvent")
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

func eventPairJSON(from, to string) ([]byte, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.eventPairJSON")
	b, err := json.Marshal(map[string]string{"from": from, "to": to})
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	return b, nil
}
