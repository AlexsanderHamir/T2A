package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
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
// Stage 2 only writes task_cycles; Stage 3 will append the matching
// cycle_started mirror row to task_events in the same SQL transaction.
func (s *Store) StartCycle(ctx context.Context, in StartCycleInput) (*domain.TaskCycle, error) {
	defer deferStoreLatency(storeOpStartCycle)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.StartCycle")
	if err := validateActor(in.TriggeredBy); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(in.TaskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: task_id", domain.ErrInvalidInput)
	}
	meta := normalizeJSONObject(in.Meta)
	var created *domain.TaskCycle
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := txLoadTask(tx, taskID); err != nil {
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
// reason is recorded in Stage 3 as part of the cycle_failed / cycle_completed
// mirror payload; for now it is validated and surfaced as an argument so the
// API surface is stable across stages.
func (s *Store) TerminateCycle(ctx context.Context, cycleID string, status domain.CycleStatus, reason string) (*domain.TaskCycle, error) {
	defer deferStoreLatency(storeOpTerminateCycle)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TerminateCycle")
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if !validTerminalCycleStatus(status) {
		return nil, fmt.Errorf("%w: status must be a terminal cycle status", domain.ErrInvalidInput)
	}
	_ = reason // recorded by Stage 3 mirror writer
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
	defer deferStoreLatency(storeOpGetCycle)()
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
	defer deferStoreLatency(storeOpListCyclesForTask)()
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
		if _, err := txLoadTask(tx, taskID); err != nil {
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

// normalizeJSONObject mirrors appendEvent's nil-data handling: a nil or
// empty payload becomes the canonical "{}" JSON object so the column's
// NOT NULL DEFAULT '{}' invariant holds even before Stage 3 wires real
// payloads through.
func normalizeJSONObject(b []byte) []byte {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.normalizeJSONObject")
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}
