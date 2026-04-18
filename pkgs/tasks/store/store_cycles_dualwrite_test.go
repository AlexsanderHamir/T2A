package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// Stage 3 invariant: every cycle/phase mutation appends a matching
// task_events audit row in the same SQL transaction. The tests below pin
// that contract end-to-end: payload shape, actor mirroring, monotonic
// task_events.seq across mixed operations, event_seq backfill on phases,
// and atomic rollback when the mirror insert itself fails.
//
// Adding a new cycle/phase mutation? Add it to this table and the audit
// row assertions force you to wire the mirror at the same time.

type cycleEventCheck struct {
	wantType    domain.EventType
	wantBy      domain.Actor
	wantPayload map[string]any
}

func decodeEventPayload(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode event payload %q: %v", string(raw), err)
	}
	return m
}

func assertSubset(t *testing.T, got, want map[string]any, label string) {
	t.Helper()
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Fatalf("%s: missing key %q in payload %v", label, k, got)
		}
		// JSON numbers decode as float64; normalize int comparisons.
		if wn, ok := wv.(int); ok {
			gn, ok := gv.(float64)
			if !ok {
				t.Fatalf("%s: key %q expected number, got %T (%v)", label, k, gv, gv)
			}
			if int(gn) != wn {
				t.Fatalf("%s: key %q = %v, want %d", label, k, gv, wn)
			}
			continue
		}
		if fmt.Sprint(gv) != fmt.Sprint(wv) {
			t.Fatalf("%s: key %q = %v, want %v", label, k, gv, wv)
		}
	}
}

func loadEventBySeq(t *testing.T, db *gorm.DB, taskID string, seq int64) domain.TaskEvent {
	t.Helper()
	var ev domain.TaskEvent
	if err := db.Where("task_id = ? AND seq = ?", taskID, seq).First(&ev).Error; err != nil {
		t.Fatalf("load event task=%s seq=%d: %v", taskID, seq, err)
	}
	return ev
}

func assertEvent(t *testing.T, db *gorm.DB, taskID string, seq int64, want cycleEventCheck) domain.TaskEvent {
	t.Helper()
	ev := loadEventBySeq(t, db, taskID, seq)
	if ev.Type != want.wantType {
		t.Fatalf("seq=%d type=%q, want %q", seq, ev.Type, want.wantType)
	}
	if ev.By != want.wantBy {
		t.Fatalf("seq=%d by=%q, want %q", seq, ev.By, want.wantBy)
	}
	got := decodeEventPayload(t, []byte(ev.Data))
	assertSubset(t, got, want.wantPayload, fmt.Sprintf("event seq=%d", seq))
	return ev
}

func TestStore_DualWrite_StartCycle_emits_cycle_started(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	beforeSeq, _ := lastEventSeqRaw(t, db, tsk.ID)
	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}

	assertEvent(t, db, tsk.ID, beforeSeq+1, cycleEventCheck{
		wantType: domain.EventCycleStarted,
		wantBy:   domain.ActorAgent,
		wantPayload: map[string]any{
			"cycle_id":     cyc.ID,
			"attempt_seq":  int(cyc.AttemptSeq),
			"triggered_by": "agent",
		},
	})
}

func TestStore_DualWrite_StartCycle_includes_parent_when_set(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	parent, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, "first attempt", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	pid := parent.ID
	child, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent, ParentCycleID: &pid})
	if err != nil {
		t.Fatal(err)
	}

	startedSeq, _ := lastEventSeqRaw(t, db, tsk.ID)
	got := decodeEventPayload(t, []byte(loadEventBySeq(t, db, tsk.ID, startedSeq).Data))
	if got["parent_cycle_id"] != parent.ID {
		t.Fatalf("parent_cycle_id = %v, want %s", got["parent_cycle_id"], parent.ID)
	}
	if got["cycle_id"] != child.ID {
		t.Fatalf("cycle_id = %v, want %s", got["cycle_id"], child.ID)
	}
}

func TestStore_DualWrite_TerminateCycle_emits_completed_or_failed(t *testing.T) {
	cases := []struct {
		name       string
		status     domain.CycleStatus
		reason     string
		wantType   domain.EventType
		wantStatus string
		wantReason string
	}{
		{"succeeded", domain.CycleStatusSucceeded, "", domain.EventCycleCompleted, "succeeded", ""},
		{"failed", domain.CycleStatusFailed, "checks didn't pass", domain.EventCycleFailed, "failed", "checks didn't pass"},
		{"aborted", domain.CycleStatusAborted, "user cancelled", domain.EventCycleFailed, "aborted", "user cancelled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := tasktestdb.OpenSQLite(t)
			s := NewStore(db)
			ctx := context.Background()
			tsk := mustCreateTask(t, s, ctx)
			cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
			if err != nil {
				t.Fatal(err)
			}

			if _, err := s.TerminateCycle(ctx, cyc.ID, tc.status, tc.reason, domain.ActorUser); err != nil {
				t.Fatal(err)
			}

			seq, _ := lastEventSeqRaw(t, db, tsk.ID)
			payload := map[string]any{
				"cycle_id":    cyc.ID,
				"attempt_seq": int(cyc.AttemptSeq),
				"status":      tc.wantStatus,
			}
			if tc.wantReason != "" {
				payload["reason"] = tc.wantReason
			}
			assertEvent(t, db, tsk.ID, seq, cycleEventCheck{
				wantType:    tc.wantType,
				wantBy:      domain.ActorUser,
				wantPayload: payload,
			})
		})
	}
}

func TestStore_DualWrite_StartPhase_backfills_event_seq(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)
	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	ph, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if ph.EventSeq == nil {
		t.Fatal("phase.EventSeq nil after StartPhase")
	}

	ev := assertEvent(t, db, tsk.ID, *ph.EventSeq, cycleEventCheck{
		wantType: domain.EventPhaseStarted,
		wantBy:   domain.ActorAgent,
		wantPayload: map[string]any{
			"cycle_id":  cyc.ID,
			"phase":     "diagnose",
			"phase_seq": int(ph.PhaseSeq),
		},
	})
	if ev.Seq != *ph.EventSeq {
		t.Fatalf("event_seq = %d, mirror seq = %d", *ph.EventSeq, ev.Seq)
	}
}

func TestStore_DualWrite_CompletePhase_emits_terminal_mirror_and_updates_event_seq(t *testing.T) {
	cases := []struct {
		name     string
		status   domain.PhaseStatus
		summary  string
		wantType domain.EventType
	}{
		{"succeeded", domain.PhaseStatusSucceeded, "diagnosed scope", domain.EventPhaseCompleted},
		{"failed", domain.PhaseStatusFailed, "verify failed", domain.EventPhaseFailed},
		{"skipped", domain.PhaseStatusSkipped, "no checks needed", domain.EventPhaseSkipped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := tasktestdb.OpenSQLite(t)
			s := NewStore(db)
			ctx := context.Background()
			tsk := mustCreateTask(t, s, ctx)
			cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
			if err != nil {
				t.Fatal(err)
			}
			ph, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
			if err != nil {
				t.Fatal(err)
			}
			startedEventSeq := *ph.EventSeq

			summary := tc.summary
			done, err := s.CompletePhase(ctx, CompletePhaseInput{
				CycleID:  cyc.ID,
				PhaseSeq: ph.PhaseSeq,
				Status:   tc.status,
				Summary:  &summary,
				By:       domain.ActorAgent,
			})
			if err != nil {
				t.Fatal(err)
			}
			if done.EventSeq == nil {
				t.Fatal("phase.EventSeq nil after CompletePhase")
			}
			if *done.EventSeq <= startedEventSeq {
				t.Fatalf("event_seq did not advance: completed=%d started=%d", *done.EventSeq, startedEventSeq)
			}

			assertEvent(t, db, tsk.ID, *done.EventSeq, cycleEventCheck{
				wantType: tc.wantType,
				wantBy:   domain.ActorAgent,
				wantPayload: map[string]any{
					"cycle_id":  cyc.ID,
					"phase":     "diagnose",
					"phase_seq": int(ph.PhaseSeq),
					"status":    string(tc.status),
					"summary":   tc.summary,
				},
			})
		})
	}
}

func TestStore_DualWrite_seq_is_monotonic_across_entrypoints(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	// task_created (seq 1) is the only baseline event seeded by Create.
	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	ph, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: cyc.ID, PhaseSeq: ph.PhaseSeq, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, cyc.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	var rows []domain.TaskEvent
	if err := db.Where("task_id = ?", tsk.ID).Order("seq ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	wantTypes := []domain.EventType{
		domain.EventTaskCreated,
		domain.EventCycleStarted,
		domain.EventPhaseStarted,
		domain.EventPhaseCompleted,
		domain.EventCycleCompleted,
	}
	if len(rows) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d (%+v)", len(rows), len(wantTypes), rows)
	}
	for i, r := range rows {
		if r.Seq != int64(i+1) {
			t.Fatalf("row %d seq = %d, want %d", i, r.Seq, i+1)
		}
		if r.Type != wantTypes[i] {
			t.Fatalf("row %d type = %q, want %q", i, r.Type, wantTypes[i])
		}
	}
}

func TestStore_DualWrite_StartCycle_rolls_back_when_mirror_insert_fails(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	// Sabotage the next task_events insert so the mirror append fails inside
	// the same transaction as the cycle insert. The cycle write must be
	// rolled back: zero new task_cycles rows AND the task_events stream
	// stays at its baseline length.
	const cb = "test_block_task_events_insert"
	if err := db.Callback().Create().Before("gorm:create").Register(cb, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_events" {
			_ = tx.AddError(errors.New("forced failure: task_events"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Create().Remove(cb); err != nil {
			t.Logf("remove callback: %v", err)
		}
	})

	beforeCycles := countRows(t, db, &domain.TaskCycle{})
	beforeEvents := countRows(t, db, &domain.TaskEvent{})

	_, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err == nil {
		t.Fatal("StartCycle: want error from mirror failure, got nil")
	}

	if got := countRows(t, db, &domain.TaskCycle{}); got != beforeCycles {
		t.Fatalf("task_cycles count = %d, want %d (cycle insert leaked despite mirror failure)", got, beforeCycles)
	}
	if got := countRows(t, db, &domain.TaskEvent{}); got != beforeEvents {
		t.Fatalf("task_events count = %d, want %d (rollback did not restore baseline)", got, beforeEvents)
	}
}

func TestStore_DualWrite_mirror_event_types_reject_user_response(t *testing.T) {
	mirrorTypes := []domain.EventType{
		domain.EventCycleStarted,
		domain.EventCycleCompleted,
		domain.EventCycleFailed,
		domain.EventPhaseStarted,
		domain.EventPhaseCompleted,
		domain.EventPhaseFailed,
		domain.EventPhaseSkipped,
	}
	for _, et := range mirrorTypes {
		if domain.EventTypeAcceptsUserResponse(et) {
			t.Fatalf("EventTypeAcceptsUserResponse(%q) = true; mirror events are observational and must reject user_response writes", et)
		}
	}
}

func lastEventSeqRaw(t *testing.T, db *gorm.DB, taskID string) (int64, error) {
	t.Helper()
	var max int64
	if err := db.Raw(`SELECT COALESCE(MAX(seq), 0) FROM task_events WHERE task_id = ?`, taskID).Scan(&max).Error; err != nil {
		t.Fatalf("last event seq: %v", err)
	}
	return max, nil
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}
