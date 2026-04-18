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
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// StartPhase appends a new phase row to a running cycle. PhaseSeq is assigned
// by the store (max + 1 within the cycle). Enforces:
//   - cycle exists and is itself running (terminal cycles are read-only);
//   - "at most one running phase per cycle" via in-TX guard;
//   - the requested next phase is allowed by the state machine in
//     domain.ValidPhaseTransition, where the previous phase is the highest-seq
//     phase already on this cycle (empty if none).
//
// In the same SQL transaction the call appends an EventPhaseStarted mirror
// row to task_events and writes the assigned task_events.seq back into the
// phase row's event_seq column so the audit pointer is one-shot.
func (s *Store) StartPhase(ctx context.Context, cycleID string, phase domain.Phase, by domain.Actor) (*domain.TaskCyclePhase, error) {
	defer deferStoreLatency(storeOpStartCyclePhase)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.StartPhase")
	if err := validateActor(by); err != nil {
		return nil, err
	}
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if !validPhase(phase) {
		return nil, fmt.Errorf("%w: phase", domain.ErrInvalidInput)
	}
	var created *domain.TaskCyclePhase
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := loadCycleByIDTx(tx, cycleID)
		if err != nil {
			return err
		}
		if domain.TerminalCycleStatus(cycle.Status) {
			return fmt.Errorf("%w: cycle is terminal", domain.ErrInvalidInput)
		}
		if err := assertNoRunningPhaseForCycleTx(tx, cycle.ID); err != nil {
			return err
		}
		prev, err := lastPhaseKindForCycleTx(tx, cycle.ID)
		if err != nil {
			return err
		}
		if !domain.ValidPhaseTransition(prev, phase) {
			return fmt.Errorf("%w: phase transition %q -> %q not allowed", domain.ErrInvalidInput, prev, phase)
		}
		nextSeq, err := nextPhaseSeqTx(tx, cycle.ID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		row := &domain.TaskCyclePhase{
			ID:          uuid.NewString(),
			CycleID:     cycle.ID,
			Phase:       phase,
			PhaseSeq:    nextSeq,
			Status:      domain.PhaseStatusRunning,
			StartedAt:   now,
			DetailsJSON: datatypes.JSON([]byte("{}")),
		}
		if err := tx.Omit("Cycle").Create(row).Error; err != nil {
			return fmt.Errorf("insert task_cycle_phase: %w", err)
		}
		evSeq, err := nextEventSeq(tx, cycle.TaskID)
		if err != nil {
			return err
		}
		payload, err := phaseStartedPayload(cycle.ID, row)
		if err != nil {
			return err
		}
		if err := appendEvent(tx, cycle.TaskID, evSeq, domain.EventPhaseStarted, by, payload); err != nil {
			return err
		}
		if err := tx.Model(&domain.TaskCyclePhase{}).Where("id = ?", row.ID).Update("event_seq", evSeq).Error; err != nil {
			return fmt.Errorf("backfill phase event_seq: %w", err)
		}
		row.EventSeq = &evSeq
		created = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// CompletePhaseInput captures the terminal transition for a phase row,
// keyed by (cycleID, phaseSeq) so the URL-level identifier from Stage 4
// (`/cycles/{cycleId}/phases/{phaseSeq}`) is also the natural store key.
type CompletePhaseInput struct {
	CycleID  string
	PhaseSeq int64
	Status   domain.PhaseStatus
	// Summary is a short human-readable note (nil to leave the column null).
	Summary *string
	// Details is structured per-phase output (verify checks, persist artifact
	// ids, …). nil/empty become the zero JSON object "{}".
	Details []byte
	// By identifies who recorded the terminal transition; mirrored as the
	// Actor on the audit row in task_events.
	By domain.Actor
}

// CompletePhase moves a running phase to a terminal status. Rejected when
// the phase row is missing, already terminal, or the requested status is not
// terminal. Does not move the parent cycle into a terminal status — that is
// an explicit TerminateCycle call so the caller controls when an attempt is
// declared finished.
//
// In the same SQL transaction the call appends an audit mirror to
// task_events (EventPhaseCompleted / EventPhaseFailed / EventPhaseSkipped
// depending on the terminal status) and writes the assigned task_events.seq
// back into the phase row's event_seq column, replacing the EventPhaseStarted
// pointer set at StartPhase time.
func (s *Store) CompletePhase(ctx context.Context, in CompletePhaseInput) (*domain.TaskCyclePhase, error) {
	defer deferStoreLatency(storeOpCompleteCyclePhase)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CompletePhase")
	if err := validateActor(in.By); err != nil {
		return nil, err
	}
	cycleID := strings.TrimSpace(in.CycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if in.PhaseSeq <= 0 {
		return nil, fmt.Errorf("%w: phase_seq", domain.ErrInvalidInput)
	}
	if !validTerminalPhaseStatus(in.Status) {
		return nil, fmt.Errorf("%w: status must be a terminal phase status", domain.ErrInvalidInput)
	}
	details := normalizeJSONObject(in.Details)
	var out *domain.TaskCyclePhase
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := loadCycleByIDTx(tx, cycleID)
		if err != nil {
			return err
		}
		ph, err := loadPhaseByCycleSeqTx(tx, cycleID, in.PhaseSeq)
		if err != nil {
			return err
		}
		if domain.TerminalPhaseStatus(ph.Status) {
			return fmt.Errorf("%w: phase already terminal", domain.ErrInvalidInput)
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"status":       in.Status,
			"ended_at":     now,
			"details_json": datatypes.JSON(details),
		}
		if in.Summary != nil {
			updates["summary"] = *in.Summary
		}
		if err := tx.Model(&domain.TaskCyclePhase{}).Where("id = ?", ph.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update task_cycle_phase: %w", err)
		}
		ph.Status = in.Status
		ph.EndedAt = &now
		ph.DetailsJSON = datatypes.JSON(details)
		if in.Summary != nil {
			s := *in.Summary
			ph.Summary = &s
		}
		evSeq, err := nextEventSeq(tx, cycle.TaskID)
		if err != nil {
			return err
		}
		payload, err := phaseTerminatedPayload(cycle.ID, ph)
		if err != nil {
			return err
		}
		mirrorType := mirrorEventTypeForPhaseStatus(in.Status)
		if err := appendEvent(tx, cycle.TaskID, evSeq, mirrorType, in.By, payload); err != nil {
			return err
		}
		if err := tx.Model(&domain.TaskCyclePhase{}).Where("id = ?", ph.ID).Update("event_seq", evSeq).Error; err != nil {
			return fmt.Errorf("backfill phase event_seq: %w", err)
		}
		ph.EventSeq = &evSeq
		out = ph
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListPhasesForCycle returns phases for cycleID in execution order
// (phase_seq ASC). The cycle must exist; an empty result for an existing
// cycle (no phases started yet) is not an error.
func (s *Store) ListPhasesForCycle(ctx context.Context, cycleID string) ([]domain.TaskCyclePhase, error) {
	defer deferStoreLatency(storeOpListCyclePhases)()
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListPhasesForCycle")
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var out []domain.TaskCyclePhase
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadCycleByIDTx(tx, cycleID); err != nil {
			return err
		}
		if err := tx.Where("cycle_id = ?", cycleID).Order("phase_seq ASC").Find(&out).Error; err != nil {
			return fmt.Errorf("list task_cycle_phases: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func loadPhaseByCycleSeqTx(tx *gorm.DB, cycleID string, phaseSeq int64) (*domain.TaskCyclePhase, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.loadPhaseByCycleSeqTx")
	var p domain.TaskCyclePhase
	if err := tx.Where("cycle_id = ? AND phase_seq = ?", cycleID, phaseSeq).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("load task_cycle_phase: %w", err)
	}
	return &p, nil
}

func assertNoRunningPhaseForCycleTx(tx *gorm.DB, cycleID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.assertNoRunningPhaseForCycleTx")
	var n int64
	if err := tx.Model(&domain.TaskCyclePhase{}).Where("cycle_id = ? AND status = ?", cycleID, domain.PhaseStatusRunning).Count(&n).Error; err != nil {
		return fmt.Errorf("running phase lookup: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%w: cycle already has a running phase", domain.ErrInvalidInput)
	}
	return nil
}

func nextPhaseSeqTx(tx *gorm.DB, cycleID string) (int64, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.nextPhaseSeqTx")
	var max int64
	if err := tx.Raw(`SELECT COALESCE(MAX(phase_seq), 0) FROM task_cycle_phases WHERE cycle_id = ?`, cycleID).Scan(&max).Error; err != nil {
		return 0, fmt.Errorf("next phase_seq: %w", err)
	}
	return max + 1, nil
}

// lastPhaseKindForCycleTx returns the Phase value of the highest-seq phase
// row in this cycle, or "" when none exist. Used to decide whether the next
// requested phase satisfies domain.ValidPhaseTransition.
func lastPhaseKindForCycleTx(tx *gorm.DB, cycleID string) (domain.Phase, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.lastPhaseKindForCycleTx")
	var p domain.TaskCyclePhase
	err := tx.Where("cycle_id = ?", cycleID).Order("phase_seq DESC").Limit(1).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("last phase lookup: %w", err)
	}
	return p.Phase, nil
}

// phaseStartedPayload builds the data_json payload for the EventPhaseStarted
// audit mirror.
func phaseStartedPayload(cycleID string, p *domain.TaskCyclePhase) ([]byte, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.phaseStartedPayload")
	out := map[string]any{
		"cycle_id":  cycleID,
		"phase":     string(p.Phase),
		"phase_seq": p.PhaseSeq,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal phase_started payload: %w", err)
	}
	return b, nil
}

// phaseTerminatedPayload builds the data_json payload for the
// EventPhaseCompleted / EventPhaseFailed / EventPhaseSkipped audit mirror.
func phaseTerminatedPayload(cycleID string, p *domain.TaskCyclePhase) ([]byte, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.phaseTerminatedPayload")
	out := map[string]any{
		"cycle_id":  cycleID,
		"phase":     string(p.Phase),
		"phase_seq": p.PhaseSeq,
		"status":    string(p.Status),
	}
	if p.Summary != nil && *p.Summary != "" {
		out["summary"] = *p.Summary
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal phase_terminated payload: %w", err)
	}
	return b, nil
}

// mirrorEventTypeForPhaseStatus picks which audit row type to write when a
// phase reaches the given terminal status.
func mirrorEventTypeForPhaseStatus(s domain.PhaseStatus) domain.EventType {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.mirrorEventTypeForPhaseStatus")
	switch s {
	case domain.PhaseStatusSucceeded:
		return domain.EventPhaseCompleted
	case domain.PhaseStatusFailed:
		return domain.EventPhaseFailed
	case domain.PhaseStatusSkipped:
		return domain.EventPhaseSkipped
	default:
		return domain.EventPhaseFailed
	}
}
