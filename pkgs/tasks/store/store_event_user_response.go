package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxTaskEventMessageBytes = 10_000
	maxResponseThreadEntries = 200
)

func parseResponseThreadJSON(raw []byte) ([]domain.ResponseThreadEntry, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.parseResponseThreadJSON")
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []domain.ResponseThreadEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("response_thread_json: %w", err)
	}
	return out, nil
}

// ThreadEntriesForDisplay returns the conversation for API/list UI, including legacy rows
// that only have user_response / user_response_at populated.
func ThreadEntriesForDisplay(ev *domain.TaskEvent) []domain.ResponseThreadEntry {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ThreadEntriesForDisplay")
	if ev == nil {
		return nil
	}
	thread, err := parseResponseThreadJSON(ev.ResponseThread)
	if err != nil {
		return nil
	}
	if len(thread) > 0 {
		return thread
	}
	if ev.UserResponse != nil {
		u := strings.TrimSpace(*ev.UserResponse)
		if u != "" {
			at := time.Now().UTC()
			if ev.UserResponseAt != nil {
				at = *ev.UserResponseAt
			}
			return []domain.ResponseThreadEntry{{At: at, By: domain.ActorUser, Body: u}}
		}
	}
	return nil
}

// AppendTaskEventResponseMessage appends one message to the event thread (user or agent).
// Event types must accept responses (see domain.EventTypeAcceptsUserResponse).
// user_response / user_response_at are synced to the latest user message in the thread.
func (s *Store) AppendTaskEventResponseMessage(ctx context.Context, taskID string, seq int64, text string, by domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AppendTaskEventResponseMessage")
	if by != domain.ActorUser && by != domain.ActorAgent {
		return fmt.Errorf("%w: by must be user or agent", domain.ErrInvalidInput)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("%w: message cannot be empty", domain.ErrInvalidInput)
	}
	if len(text) > maxTaskEventMessageBytes {
		return fmt.Errorf("%w: message too long (max %d bytes)", domain.ErrInvalidInput, maxTaskEventMessageBytes)
	}
	tid := strings.TrimSpace(taskID)
	if tid == "" {
		return fmt.Errorf("%w: id", domain.ErrInvalidInput)
	}
	if seq < 1 {
		return fmt.Errorf("%w: seq", domain.ErrInvalidInput)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ev domain.TaskEvent
		q := tx.Where("task_id = ? AND seq = ?", tid, seq)
		// SQLite test driver deadlocks on SELECT … FOR UPDATE under concurrent writers;
		// production Postgres benefits from row locking to prevent lost updates.
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
	})
}
