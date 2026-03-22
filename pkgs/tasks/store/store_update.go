package store

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

func applyTaskPatches(tx *gorm.DB, taskID string, cur *domain.Task, in UpdateTaskInput, by domain.Actor, seq int64) error {
	if in.Title != nil {
		v := strings.TrimSpace(*in.Title)
		if v == "" {
			return fmt.Errorf("%w: title", domain.ErrInvalidInput)
		}
		if v != cur.Title {
			b, err := eventPairJSON(cur.Title, v)
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventMessageAdded, by, b); err != nil {
				return err
			}
			seq++
			cur.Title = v
		}
	}
	if in.InitialPrompt != nil {
		if *in.InitialPrompt != cur.InitialPrompt {
			b, err := eventPairJSON(cur.InitialPrompt, *in.InitialPrompt)
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventPromptAppended, by, b); err != nil {
				return err
			}
			seq++
			cur.InitialPrompt = *in.InitialPrompt
		}
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return fmt.Errorf("%w: status", domain.ErrInvalidInput)
		}
		if *in.Status != cur.Status {
			b, err := eventPairJSON(string(cur.Status), string(*in.Status))
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventStatusChanged, by, b); err != nil {
				return err
			}
			seq++
			cur.Status = *in.Status
		}
	}
	if in.Priority != nil {
		if !validPriority(*in.Priority) {
			return fmt.Errorf("%w: priority", domain.ErrInvalidInput)
		}
		if *in.Priority != cur.Priority {
			b, err := eventPairJSON(string(cur.Priority), string(*in.Priority))
			if err != nil {
				return err
			}
			if err := appendEvent(tx, taskID, seq, domain.EventPriorityChanged, by, b); err != nil {
				return err
			}
			seq++
			cur.Priority = *in.Priority
		}
	}
	return nil
}
