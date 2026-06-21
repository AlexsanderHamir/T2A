package scheduling

import (
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestDecideNotifyAfterReadyTransition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Minute)

	readyTask := &domain.Task{ID: "t1", Status: domain.StatusReady}
	futurePickup := &domain.Task{ID: "t2", Status: domain.StatusReady, PickupNotBefore: &future}
	eligiblePickup := &domain.Task{ID: "t3", Status: domain.StatusReady, PickupNotBefore: &past}

	cases := []struct {
		name          string
		prev          domain.Status
		task          *domain.Task
		pickupTouched bool
		wantNotify    bool
		wantWake      bool
		wantCancel    bool
	}{
		{"create ready", "", readyTask, false, true, false, true},
		{"transition to ready", domain.StatusBlocked, readyTask, false, true, false, true},
		{"stay ready no pickup touch", domain.StatusReady, readyTask, false, false, false, true},
		{"pickup touched eligible", domain.StatusReady, eligiblePickup, true, true, false, true},
		{"future pickup schedules wake", domain.StatusBlocked, futurePickup, false, false, true, false},
		{"not ready cancels wake", domain.StatusBlocked, &domain.Task{Status: domain.StatusBlocked}, false, false, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DecideNotifyAfterReadyTransition(c.prev, c.task, c.pickupTouched, now)
			if got.NotifyQueue != c.wantNotify {
				t.Fatalf("NotifyQueue=%v want %v", got.NotifyQueue, c.wantNotify)
			}
			if (got.ScheduleWake != nil) != c.wantWake {
				t.Fatalf("ScheduleWake set=%v want %v", got.ScheduleWake != nil, c.wantWake)
			}
			if got.CancelWake != c.wantCancel {
				t.Fatalf("CancelWake=%v want %v", got.CancelWake, c.wantCancel)
			}
		})
	}
}
