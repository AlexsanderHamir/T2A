package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func appendResponseMessageInTx(tx *gorm.DB, tid string, seq int64, text string, by domain.Actor) error {
	var ev domain.TaskEvent
	q := tx.Where("task_id = ? AND seq = ?", tid, seq)
	if tx.Dialector.Name() != "sqlite" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("load task event: %w", err)
	}
	if !domain.EventTypeAcceptsUserResponse(ev.Type) {
		return fmt.Errorf("%w: this event type does not accept thread messages", domain.ErrInvalidInput)
	}
	thread, err := parseResponseThreadJSON(ev.ResponseThread)
	if err != nil {
		return err
	}
	if len(thread) == 0 && ev.UserResponse != nil {
		u := strings.TrimSpace(*ev.UserResponse)
		if u != "" {
			at := time.Now().UTC()
			if ev.UserResponseAt != nil {
				at = *ev.UserResponseAt
			}
			thread = []domain.ResponseThreadEntry{{At: at, By: domain.ActorUser, Body: u}}
		}
	}
	if len(thread) >= maxResponseThreadEntries {
		return fmt.Errorf("%w: thread is full (max %d messages)", domain.ErrInvalidInput, maxResponseThreadEntries)
	}
	now := time.Now().UTC()
	thread = append(thread, domain.ResponseThreadEntry{At: now, By: by, Body: text})
	raw, err := json.Marshal(thread)
	if err != nil {
		return fmt.Errorf("marshal response thread: %w", err)
	}
	var userResp *string
	var userAt *time.Time
	for i := len(thread) - 1; i >= 0; i-- {
		if thread[i].By == domain.ActorUser {
			b := thread[i].Body
			userResp = &b
			t := thread[i].At.UTC()
			userAt = &t
			break
		}
	}
	if err := tx.Model(&domain.TaskEvent{}).
		Where("task_id = ? AND seq = ?", tid, seq).
		Updates(map[string]any{
			"response_thread_json": raw,
			"user_response":        userResp,
			"user_response_at":     userAt,
		}).Error; err != nil {
		return fmt.Errorf("update task event response thread: %w", err)
	}
	return nil
}
