package cycles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// maxPhaseEventDetailRunes caps each string value inside the optional
// "details" object attached to phase_* audit events so a pathological
// runner payload cannot blow up SSE payloads.
const maxPhaseEventDetailRunes = 8192

// StartPhase appends a new phase row to a running cycle. PhaseSeq is
// assigned by the store (max + 1 within the cycle). Enforces:
//   - cycle exists and is itself running (terminal cycles are read-only);
//   - "at most one running phase per cycle" via in-TX guard;
//   - the requested next phase is allowed by the state machine in
//     domain.ValidPhaseTransition, where the previous phase is the
//     highest-seq phase already on this cycle (empty if none).
//
// In the same SQL transaction the call appends an EventPhaseStarted
// mirror row to task_events and writes the assigned task_events.seq
// back into the phase row's event_seq column so the audit pointer is
// one-shot.
func StartPhase(ctx context.Context, db *gorm.DB, cycleID string, phase domain.Phase, by domain.Actor) (*domain.TaskCyclePhase, error) {
	defer kernel.DeferLatency(kernel.OpStartCyclePhase)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.StartPhase")
	if err := kernel.ValidateActor(by); err != nil {
		return nil, err
	}
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if !kernel.ValidPhase(phase) {
		return nil, fmt.Errorf("%w: phase", domain.ErrInvalidInput)
	}
	var created *domain.TaskCyclePhase
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := loadByIDInTx(tx, cycleID)
		if err != nil {
			return err
		}
		if domain.TerminalCycleStatus(cycle.Status) {
			return fmt.Errorf("%w: cycle is terminal", domain.ErrInvalidInput)
		}
		if err := assertNoRunningPhaseForCycleInTx(tx, cycle.ID); err != nil {
			return err
		}
		prev, err := lastPhaseKindForCycleInTx(tx, cycle.ID)
		if err != nil {
			return err
		}
		if !domain.ValidPhaseTransition(prev, phase) {
			return fmt.Errorf("%w: phase transition %q -> %q not allowed", domain.ErrInvalidInput, prev, phase)
		}
		nextSeq, err := nextPhaseSeqInTx(tx, cycle.ID)
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
		evSeq, err := kernel.NextEventSeq(tx, cycle.TaskID)
		if err != nil {
			return err
		}
		payload, err := phaseStartedPayload(cycle.ID, row)
		if err != nil {
			return err
		}
		if err := kernel.AppendEvent(tx, cycle.TaskID, evSeq, domain.EventPhaseStarted, by, payload); err != nil {
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

// CompletePhase moves a running phase to a terminal status. Rejected
// when the phase row is missing, already terminal, or the requested
// status is not terminal. Does not move the parent cycle into a
// terminal status — that is an explicit Terminate call so the caller
// controls when an attempt is declared finished.
//
// In the same SQL transaction the call appends an audit mirror to
// task_events (EventPhaseCompleted / EventPhaseFailed /
// EventPhaseSkipped depending on the terminal status) and writes the
// assigned task_events.seq back into the phase row's event_seq column,
// replacing the EventPhaseStarted pointer set at StartPhase time.
func CompletePhase(ctx context.Context, db *gorm.DB, in CompletePhaseInput) (*domain.TaskCyclePhase, error) {
	defer kernel.DeferLatency(kernel.OpCompleteCyclePhase)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.CompletePhase")
	if err := kernel.ValidateActor(in.By); err != nil {
		return nil, err
	}
	cycleID := strings.TrimSpace(in.CycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	if in.PhaseSeq <= 0 {
		return nil, fmt.Errorf("%w: phase_seq", domain.ErrInvalidInput)
	}
	if !kernel.ValidTerminalPhaseStatus(in.Status) {
		return nil, fmt.Errorf("%w: status must be a terminal phase status", domain.ErrInvalidInput)
	}
	details, err := kernel.NormalizeJSONObject(in.Details, "details")
	if err != nil {
		return nil, err
	}
	var out *domain.TaskCyclePhase
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := loadByIDInTx(tx, cycleID)
		if err != nil {
			return err
		}
		ph, err := loadPhaseByCycleSeqInTx(tx, cycleID, in.PhaseSeq)
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
		evSeq, err := kernel.NextEventSeq(tx, cycle.TaskID)
		if err != nil {
			return err
		}
		payload, err := phaseTerminatedPayload(cycle.ID, ph)
		if err != nil {
			return err
		}
		mirrorType := mirrorEventTypeForPhaseStatus(in.Status)
		if err := kernel.AppendEvent(tx, cycle.TaskID, evSeq, mirrorType, in.By, payload); err != nil {
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
// (phase_seq ASC). The cycle must exist; an empty result for an
// existing cycle (no phases started yet) is not an error.
func ListPhasesForCycle(ctx context.Context, db *gorm.DB, cycleID string) ([]domain.TaskCyclePhase, error) {
	defer kernel.DeferLatency(kernel.OpListCyclePhases)()
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.ListPhasesForCycle")
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return nil, fmt.Errorf("%w: cycle_id", domain.ErrInvalidInput)
	}
	var out []domain.TaskCyclePhase
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadByIDInTx(tx, cycleID); err != nil {
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

func loadPhaseByCycleSeqInTx(tx *gorm.DB, cycleID string, phaseSeq int64) (*domain.TaskCyclePhase, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.loadPhaseByCycleSeqInTx")
	var p domain.TaskCyclePhase
	if err := tx.Where("cycle_id = ? AND phase_seq = ?", cycleID, phaseSeq).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("load task_cycle_phase: %w", err)
	}
	return &p, nil
}

func assertNoRunningPhaseForCycleInTx(tx *gorm.DB, cycleID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.assertNoRunningPhaseForCycleInTx")
	var n int64
	if err := tx.Model(&domain.TaskCyclePhase{}).Where("cycle_id = ? AND status = ?", cycleID, domain.PhaseStatusRunning).Count(&n).Error; err != nil {
		return fmt.Errorf("running phase lookup: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%w: cycle already has a running phase", domain.ErrInvalidInput)
	}
	return nil
}

func nextPhaseSeqInTx(tx *gorm.DB, cycleID string) (int64, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.nextPhaseSeqInTx")
	var max int64
	if err := tx.Raw(`SELECT COALESCE(MAX(phase_seq), 0) FROM task_cycle_phases WHERE cycle_id = ?`, cycleID).Scan(&max).Error; err != nil {
		return 0, fmt.Errorf("next phase_seq: %w", err)
	}
	return max + 1, nil
}

// lastPhaseKindForCycleInTx returns the Phase value of the highest-seq
// phase row in this cycle, or "" when none exist. Used to decide
// whether the next requested phase satisfies
// domain.ValidPhaseTransition.
func lastPhaseKindForCycleInTx(tx *gorm.DB, cycleID string) (domain.Phase, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.lastPhaseKindForCycleInTx")
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

// phaseStartedPayload builds the data_json payload for the
// EventPhaseStarted audit mirror.
func phaseStartedPayload(cycleID string, p *domain.TaskCyclePhase) ([]byte, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.phaseStartedPayload")
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
// EventPhaseCompleted / EventPhaseFailed / EventPhaseSkipped audit
// mirror.
func phaseTerminatedPayload(cycleID string, p *domain.TaskCyclePhase) ([]byte, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.phaseTerminatedPayload")
	out := map[string]any{
		"cycle_id":  cycleID,
		"phase":     string(p.Phase),
		"phase_seq": p.PhaseSeq,
		"status":    string(p.Status),
	}
	if p.Summary != nil && *p.Summary != "" {
		out["summary"] = *p.Summary
	}
	if d := phaseDetailsForEventPayload(p.DetailsJSON); len(d) > 0 {
		out["details"] = d
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal phase_terminated payload: %w", err)
	}
	return b, nil
}

// phaseDetailsForEventPayload returns a deep-copied, size-clamped JSON object
// suitable for task_events.data_json so the timeline can show stderr tails,
// token usage, etc., without a separate cycle-phase fetch.
func phaseDetailsForEventPayload(detailsJSON datatypes.JSON) map[string]any {
	if len(detailsJSON) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(detailsJSON, &obj); err != nil || len(obj) == 0 {
		return nil
	}
	out := truncatePhaseEventDetailValue(obj, maxPhaseEventDetailRunes)
	if m, ok := out.(map[string]any); ok {
		return m
	}
	return nil
}

func truncatePhaseEventDetailValue(v any, maxRunes int) any {
	switch x := v.(type) {
	case string:
		return truncateStringRunes(x, maxRunes)
	case map[string]any:
		for k, vv := range x {
			x[k] = truncatePhaseEventDetailValue(vv, maxRunes)
		}
		return x
	case []any:
		for i, vv := range x {
			x[i] = truncatePhaseEventDetailValue(vv, maxRunes)
		}
		return x
	default:
		return v
	}
}

func truncateStringRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxRunes {
			b.WriteRune('…')
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// mirrorEventTypeForPhaseStatus picks which audit row type to write
// when a phase reaches the given terminal status.
func mirrorEventTypeForPhaseStatus(s domain.PhaseStatus) domain.EventType {
	slog.Debug("trace", "cmd", logCmd, "operation", "tasks.store.cycles.mirrorEventTypeForPhaseStatus")
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
