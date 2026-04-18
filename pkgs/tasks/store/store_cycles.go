package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// StartCycleInput captures everything needed to begin a new execution attempt
// for a task. The store decides AttemptSeq; callers cannot supply it.
type StartCycleInput struct {
	TaskID        string
	TriggeredBy   domain.Actor
	ParentCycleID *string
	// Meta is small free-form runner metadata such as
	// {"runner":"cursor-cli","prompt_hash":"..."}. nil and empty are normalized
	// to the zero JSON object "{}".
	Meta []byte
}

// StartCycle creates a new TaskCycle row with status=running for the given
// task. Enforces "at most one running cycle per task" via an in-TX guard
// (portable across Postgres + SQLite); concurrent attempts surface as
// domain.ErrInvalidInput so callers can surface a 400 to clients (a worker
// retrying through Idempotency-Key is the supported recovery path).
//
// AttemptSeq is assigned by the store: max(attempt_seq) over the task + 1.
// ParentCycleID, when non-nil, must reference an existing cycle row that
// belongs to the same task; cross-task lineage is rejected as invalid input.
//
// In the same SQL transaction the call appends an EventCycleStarted mirror
// row to task_events so GET /tasks/{id}/events stays a complete witness of
// cycle activity. If the mirror insert fails, the cycle row is rolled back.
func (s *Store) StartCycle(ctx context.Context, in StartCycleInput) (*domain.TaskCycle, error) {
	defer kernel.DeferLatency(kernel.OpStartCycle)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.StartCycle")
	if err := kernel.ValidateActor(in.TriggeredBy); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(in.TaskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: task_id", domain.ErrInvalidInput)
	}
	meta, err := normalizeJSONObject(in.Meta, "meta")
	if err != nil {
		return nil, err
	}
	var created *domain.TaskCycle
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := kernel.LoadTask(tx, taskID); err != nil {
			return err
		}
		if err := assertNoRunningCycleForTaskTx(tx, taskID); err != nil {
			return err
		}
		if in.ParentCycleID != nil {
			parentID := strings.TrimSpace(*in.ParentCycleID)
			if parentID == "" {
				return fmt.Errorf("%w: parent_cycle_id", domain.ErrInvalidInput)
			}
			parent, err := loadCycleByIDTx(tx, parentID)
			if err != nil {
				return err
			}
			if parent.TaskID != taskID {
				return fmt.Errorf("%w: parent_cycle_id does not belong to this task", domain.ErrInvalidInput)
			}
			in.ParentCycleID = &parent.ID
		}
		nextAttempt, err := nextAttemptSeqTx(tx, taskID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		row := &domain.TaskCycle{
			ID:            uuid.NewString(),
			TaskID:        taskID,
			AttemptSeq:    nextAttempt,
			Status:        domain.CycleStatusRunning,
			StartedAt:     now,
			TriggeredBy:   in.TriggeredBy,
			ParentCycleID: in.ParentCycleID,
			MetaJSON:      datatypes.JSON(meta),
		}
		if err := tx.Omit("Task").Create(row).Error; err != nil {
			return fmt.Errorf("insert task_cycle: %w", err)
		}
		seq, err := kernel.NextEventSeq(tx, taskID)
		if err != nil {
			return err
		}
		payload, err := cycleStartedPayload(row)
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, taskID, seq, domain.EventCycleStarted, in.TriggeredBy, payload); err != nil {
			return err
		}
		created = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// TerminateCycle moves a running cycle into a terminal state. Rejected when
// the cycle is already terminal, when the requested status is not terminal,
// or when the cycle still has a running phase row.
//
// In the same SQL transaction the call appends a mirror row to task_events:
// EventCycleCompleted for CycleStatusSucceeded; EventCycleFailed for
// CycleStatusFailed and CycleStatusAborted (the mirror payload's status
// field preserves the distinction between failed and aborted). reason, if
// non-empty, is included in the mirror payload.
func (s *Store) TerminateCycle(ctx context.Context, cycleID string, status domain.CycleStatus, reason string, by domain.Actor) (*domain.TaskCycle, error) {
	defer kernel.DeferLatency(kernel.OpTerminateCycle)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TerminateCycle")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, err
	}
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if !kernel.ValidTerminalCycleStatus(status) {
		return nil, fmt.Errorf("%w: status must be a terminal cycle status", domain.ErrInvalidInput)
	}
	reason = strings.TrimSpace(reason)
	var out *domain.TaskCycle
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := loadCycleByIDTx(tx, cycleID)
		if err != nil {
			return err
		}
		if domain.TerminalCycleStatus(cycle.Status) {
			return fmt.Errorf("%w: cycle already terminal", domain.ErrInvalidInput)
		}
		if err := assertNoRunningPhaseForCycleTx(tx, cycle.ID); err != nil {
			return err
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"status":   status,
			"ended_at": now,
		}
		if err := tx.Model(&domain.TaskCycle{}).Where("id = ?", cycle.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update task_cycle: %w", err)
		}
		cycle.Status = status
		cycle.EndedAt = &now
		seq, err := kernel.NextEventSeq(tx, cycle.TaskID)
		if err != nil {
			return err
		}
		payload, err := cycleTerminatedPayload(cycle, reason)
		if err != nil {
			return err
		}
		mirrorType := mirrorEventTypeForCycleStatus(status)
		if err := kernel.AppendEvent(tx, cycle.TaskID, seq, mirrorType, by, payload); err != nil {
			return err
		}
		out = cycle
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetCycle returns one cycle by id; ErrNotFound when missing.
func (s *Store) GetCycle(ctx context.Context, cycleID string) (*domain.TaskCycle, error) {
	defer kernel.DeferLatency(kernel.OpGetCycle)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetCycle")
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	return loadCycleByIDTx(s.db.WithContext(ctx), cycleID)
}

// ListCyclesForTask returns cycles for a task ordered by attempt_seq DESC
// (newest first). limit is clamped to [1, 200]; the task must exist.
func (s *Store) ListCyclesForTask(ctx context.Context, taskID string, limit int) ([]domain.TaskCycle, error) {
	defer kernel.DeferLatency(kernel.OpListCyclesForTask)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListCyclesForTask")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: task_id", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var out []domain.TaskCycle
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := kernel.LoadTask(tx, taskID); err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", taskID).Order("attempt_seq DESC").Limit(limit).Find(&out).Error; err != nil {
			return fmt.Errorf("list task_cycles: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// loadCycleByIDTx fetches one cycle by id with gorm errors mapped to the
// domain sentinels. Exported helpers and phase code share this lookup.
func loadCycleByIDTx(tx *gorm.DB, cycleID string) (*domain.TaskCycle, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.loadCycleByIDTx")
	var c domain.TaskCycle
	if err := tx.Where("id = ?", cycleID).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("load task_cycle: %w", err)
	}
	return &c, nil
}

func assertNoRunningCycleForTaskTx(tx *gorm.DB, taskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.assertNoRunningCycleForTaskTx")
	var n int64
	if err := tx.Model(&domain.TaskCycle{}).Where("task_id = ? AND status = ?", taskID, domain.CycleStatusRunning).Count(&n).Error; err != nil {
		return fmt.Errorf("running cycle lookup: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%w: task already has a running cycle", domain.ErrInvalidInput)
	}
	return nil
}

func nextAttemptSeqTx(tx *gorm.DB, taskID string) (int64, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.nextAttemptSeqTx")
	var max int64
	if err := tx.Raw(`SELECT COALESCE(MAX(attempt_seq), 0) FROM task_cycles WHERE task_id = ?`, taskID).Scan(&max).Error; err != nil {
		return 0, fmt.Errorf("next attempt_seq: %w", err)
	}
	return max + 1, nil
}

// normalizeJSONObject is the chokepoint that enforces the documented
// "meta_json / details_json columns are always a JSON object, defaulted to
// {}" invariant for cycle and phase mutations (docs/EXECUTION-CYCLES.md
// §column conventions; docs/API-HTTP.md cycle / phase routes).
//
// Inputs are normalized as follows:
//   - nil / empty bytes / whitespace-only / the JSON literal "null" all
//     collapse to the canonical zero value []byte("{}"). This matches the
//     contract that the column never carries SQL NULL or an empty string.
//   - A well-formed JSON object passes through unchanged.
//   - Anything else (string, number, array, bool, malformed JSON) is
//     rejected with domain.ErrInvalidInput wrapped with the field name so
//     handlers can surface a 400 to the client. Silent coercion would let
//     the column hold values that violate the documented shape and break
//     downstream parsers (web `parseTaskApi`, response struct contract).
func normalizeJSONObject(b []byte, field string) ([]byte, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.normalizeJSONObject")
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []byte("{}"), nil
	}
	var probe any
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return nil, fmt.Errorf("%w: %s must be a JSON object", domain.ErrInvalidInput, field)
	}
	if _, ok := probe.(map[string]any); !ok {
		return nil, fmt.Errorf("%w: %s must be a JSON object", domain.ErrInvalidInput, field)
	}
	return b, nil
}

// cycleStartedPayload builds the data_json payload for the EventCycleStarted
// audit mirror. Keys are stable (asserted by the dual-write invariant test).
func cycleStartedPayload(c *domain.TaskCycle) ([]byte, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.cycleStartedPayload")
	out := map[string]any{
		"cycle_id":     c.ID,
		"attempt_seq":  c.AttemptSeq,
		"triggered_by": string(c.TriggeredBy),
	}
	if c.ParentCycleID != nil && *c.ParentCycleID != "" {
		out["parent_cycle_id"] = *c.ParentCycleID
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal cycle_started payload: %w", err)
	}
	return b, nil
}

// cycleTerminatedPayload builds the data_json payload for the
// EventCycleCompleted / EventCycleFailed audit mirror.
func cycleTerminatedPayload(c *domain.TaskCycle, reason string) ([]byte, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.cycleTerminatedPayload")
	out := map[string]any{
		"cycle_id":    c.ID,
		"attempt_seq": c.AttemptSeq,
		"status":      string(c.Status),
	}
	if reason != "" {
		out["reason"] = reason
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal cycle_terminated payload: %w", err)
	}
	return b, nil
}

// mirrorEventTypeForCycleStatus picks which audit row type to write when a
// cycle reaches the given terminal status. CycleStatusAborted folds into
// EventCycleFailed; the payload's status field preserves the distinction.
func mirrorEventTypeForCycleStatus(s domain.CycleStatus) domain.EventType {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.mirrorEventTypeForCycleStatus")
	if s == domain.CycleStatusSucceeded {
		return domain.EventCycleCompleted
	}
	return domain.EventCycleFailed
}
