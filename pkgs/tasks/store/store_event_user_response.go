package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
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
	defer deferStoreLatency(storeOpAppendTaskEventResponse)()
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
		return appendResponseMessageInTx(tx, tid, seq, text, by)
	})
}
