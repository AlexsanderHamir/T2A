package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func nextEventSeq(tx *gorm.DB, taskID string) (int64, error) {
	var max int64
	err := tx.Raw(`SELECT COALESCE(MAX(seq), 0) FROM task_events WHERE task_id = ?`, taskID).Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("next event seq: %w", err)
	}
	return max + 1, nil
}

func appendEvent(tx *gorm.DB, taskID string, seq int64, typ EventType, by Actor, data []byte) error {
	if data == nil {
		data = []byte("{}")
	}
	ev := TaskEvent{
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
	b, err := json.Marshal(map[string]string{"from": from, "to": to})
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	return b, nil
}
