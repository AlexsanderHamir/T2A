package cycles

import (
	"context"
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

const defaultStreamEventLimit = 100
const maxStreamEventLimit = 500

// AppendStreamEvent appends one normalized runner progress event to a cycle.
func AppendStreamEvent(ctx context.Context, db *gorm.DB, in AppendStreamEventInput) (*domain.TaskCycleStreamEvent, error) {
	defer kernel.DeferLatency(kernel.OpAppendCycleStreamEvent)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.AppendStreamEvent")
	taskID := strings.TrimSpace(in.TaskID)
	cycleID := strings.TrimSpace(in.CycleID)
	source := strings.TrimSpace(in.Source)
	kind := strings.TrimSpace(in.Kind)
	if taskID == "" || cycleID == "" || in.PhaseSeq <= 0 || source == "" || kind == "" {
		return nil, fmt.Errorf("%w: stream event", domain.ErrInvalidInput)
	}
	payload, err := kernel.NormalizeJSONObject(in.Payload, "payload")
	if err != nil {
		return nil, err
	}
	at := in.At.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	var out *domain.TaskCycleStreamEvent
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := loadByIDInTx(tx, cycleID)
		if err != nil {
			return err
		}
		if cycle.TaskID != taskID {
			return domain.ErrNotFound
		}
		if _, err := loadPhaseByCycleSeqInTx(tx, cycleID, in.PhaseSeq); err != nil {
			return err
		}
		next, err := nextStreamSeqInTx(tx, cycleID)
		if err != nil {
			return err
		}
		row := &domain.TaskCycleStreamEvent{
			ID:          uuid.NewString(),
			TaskID:      taskID,
			CycleID:     cycleID,
			PhaseSeq:    in.PhaseSeq,
			StreamSeq:   next,
			At:          at,
			Source:      source,
			Kind:        kind,
			Subtype:     strings.TrimSpace(in.Subtype),
			Message:     strings.TrimSpace(in.Message),
			Tool:        strings.TrimSpace(in.Tool),
			PayloadJSON: datatypes.JSON(payload),
		}
		if err := tx.Omit("Task", "Cycle").Create(row).Error; err != nil {
			return fmt.Errorf("insert task_cycle_stream_event: %w", err)
		}
		out = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListStreamEvents returns stream events for cycleID ordered by stream_seq ASC.
func ListStreamEvents(ctx context.Context, db *gorm.DB, cycleID string, afterSeq int64, limit int) ([]domain.TaskCycleStreamEvent, error) {
	defer kernel.DeferLatency(kernel.OpListCycleStreamEvents)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.ListStreamEvents")
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if limit <= 0 {
		limit = defaultStreamEventLimit
	}
	if limit > maxStreamEventLimit {
		limit = maxStreamEventLimit
	}
	var out []domain.TaskCycleStreamEvent
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadByIDInTx(tx, cycleID); err != nil {
			return err
		}
		q := tx.Where("cycle_id = ?", cycleID)
		if afterSeq > 0 {
			q = q.Where("stream_seq > ?", afterSeq)
		}
		if err := q.Order("stream_seq ASC").Limit(limit).Find(&out).Error; err != nil {
			return fmt.Errorf("list task_cycle_stream_events: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func nextStreamSeqInTx(tx *gorm.DB, cycleID string) (int64, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.nextStreamSeqInTx")
	var max int64
	if err := tx.Raw(`SELECT COALESCE(MAX(stream_seq), 0) FROM task_cycle_stream_events WHERE cycle_id = ?`, cycleID).Scan(&max).Error; err != nil {
		return 0, fmt.Errorf("next stream_seq: %w", err)
	}
	return max + 1, nil
}
