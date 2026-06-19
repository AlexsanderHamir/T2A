package domain

import "testing"

func TestValidPhaseTransition_initialEntry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		next Phase
		want bool
	}{
		{name: "empty -> execute is the only valid start", next: PhaseExecute, want: true},
		{name: "empty -> verify rejected", next: PhaseVerify, want: false},
		{name: "empty -> empty rejected", next: "", want: false},
		{name: "empty -> unknown rejected", next: Phase("garbage"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidPhaseTransition("", tc.next); got != tc.want {
				t.Fatalf("ValidPhaseTransition(\"\", %q) = %v, want %v", tc.next, got, tc.want)
			}
		})
	}
}

func TestValidPhaseTransition_forwardEdges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		prev Phase
		next Phase
		want bool
	}{
		{name: "execute -> verify", prev: PhaseExecute, next: PhaseVerify, want: true},
		{name: "verify -> execute is the corrective re-entry edge", prev: PhaseVerify, next: PhaseExecute, want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidPhaseTransition(tc.prev, tc.next); got != tc.want {
				t.Fatalf("ValidPhaseTransition(%q, %q) = %v, want %v", tc.prev, tc.next, got, tc.want)
			}
		})
	}
}

func TestValidVerifyOnlyRetryTransition(t *testing.T) {
	t.Parallel()
	last := &TaskCyclePhase{
		Phase:  PhaseVerify,
		Status: PhaseStatusFailed,
	}
	if !ValidVerifyOnlyRetryTransition(last, PhaseVerify) {
		t.Fatal("verify→verify after terminal failed verify should be allowed")
	}
	if ValidVerifyOnlyRetryTransition(last, PhaseExecute) {
		t.Fatal("verify→execute should use ValidPhaseTransition, not verify-only helper")
	}
	running := &TaskCyclePhase{Phase: PhaseVerify, Status: PhaseStatusRunning}
	if ValidVerifyOnlyRetryTransition(running, PhaseVerify) {
		t.Fatal("running verify must not allow verify-only re-entry")
	}
}

func TestValidPhaseTransition_rejectsInvalidEdges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		prev Phase
		next Phase
	}{
		{name: "execute -> execute self-loop without verify gate", prev: PhaseExecute, next: PhaseExecute},
		{name: "verify -> verify self-loop", prev: PhaseVerify, next: PhaseVerify},
		{name: "unknown prev rejects everything", prev: Phase("garbage"), next: PhaseExecute},
		{name: "any -> empty next rejected", prev: PhaseExecute, next: ""},
		{name: "any -> unknown next rejected", prev: PhaseExecute, next: Phase("garbage")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidPhaseTransition(tc.prev, tc.next); got {
				t.Fatalf("ValidPhaseTransition(%q, %q) = true, want false", tc.prev, tc.next)
			}
		})
	}
}

func TestTerminalCycleStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   CycleStatus
		want bool
	}{
		{name: "running is not terminal", in: CycleStatusRunning, want: false},
		{name: "succeeded is terminal", in: CycleStatusSucceeded, want: true},
		{name: "failed is terminal", in: CycleStatusFailed, want: true},
		{name: "aborted is terminal", in: CycleStatusAborted, want: true},
		{name: "empty is not terminal", in: CycleStatus(""), want: false},
		{name: "unknown is not terminal", in: CycleStatus("noop"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := TerminalCycleStatus(tc.in); got != tc.want {
				t.Fatalf("TerminalCycleStatus(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTerminalPhaseStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   PhaseStatus
		want bool
	}{
		{name: "running is not terminal", in: PhaseStatusRunning, want: false},
		{name: "succeeded is terminal", in: PhaseStatusSucceeded, want: true},
		{name: "failed is terminal", in: PhaseStatusFailed, want: true},
		{name: "skipped is terminal", in: PhaseStatusSkipped, want: true},
		{name: "empty is not terminal", in: PhaseStatus(""), want: false},
		{name: "unknown is not terminal", in: PhaseStatus("noop"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := TerminalPhaseStatus(tc.in); got != tc.want {
				t.Fatalf("TerminalPhaseStatus(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestCycleAndPhaseEnumValues(t *testing.T) {
	t.Parallel()

	if string(PhaseExecute) != "execute" || string(PhaseVerify) != "verify" {
		t.Fatalf("phase enum string drift: %q %q", PhaseExecute, PhaseVerify)
	}
	if string(CycleStatusRunning) != "running" || string(CycleStatusSucceeded) != "succeeded" || string(CycleStatusFailed) != "failed" || string(CycleStatusAborted) != "aborted" {
		t.Fatalf("cycle status enum string drift: %q %q %q %q", CycleStatusRunning, CycleStatusSucceeded, CycleStatusFailed, CycleStatusAborted)
	}
	if string(PhaseStatusRunning) != "running" || string(PhaseStatusSucceeded) != "succeeded" || string(PhaseStatusFailed) != "failed" || string(PhaseStatusSkipped) != "skipped" {
		t.Fatalf("phase status enum string drift: %q %q %q %q", PhaseStatusRunning, PhaseStatusSucceeded, PhaseStatusFailed, PhaseStatusSkipped)
	}
}

func TestCycleEventTypeStringValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		got, want EventType
	}{
		{EventCycleStarted, "cycle_started"},
		{EventCycleCompleted, "cycle_completed"},
		{EventCycleFailed, "cycle_failed"},
		{EventPhaseStarted, "phase_started"},
		{EventPhaseCompleted, "phase_completed"},
		{EventPhaseFailed, "phase_failed"},
		{EventPhaseSkipped, "phase_skipped"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.want), func(t *testing.T) {
			t.Parallel()
			if string(tc.got) != string(tc.want) {
				t.Fatalf("event type drift: got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestEventTypeAcceptsUserResponse_excludesCycleAndPhaseEvents(t *testing.T) {
	t.Parallel()

	for _, et := range []EventType{
		EventCycleStarted,
		EventCycleCompleted,
		EventCycleFailed,
		EventPhaseStarted,
		EventPhaseCompleted,
		EventPhaseFailed,
		EventPhaseSkipped,
	} {
		et := et
		t.Run(string(et), func(t *testing.T) {
			t.Parallel()
			if EventTypeAcceptsUserResponse(et) {
				t.Fatalf("execution-cycle audit mirrors are observational; %q must not accept user_response", et)
			}
		})
	}
}

func TestValidInterruptResumeTransition(t *testing.T) {
	t.Parallel()
	reason := PhaseInterruptReason
	last := TaskCyclePhase{
		Phase:   PhaseExecute,
		Status:  PhaseStatusFailed,
		Summary: &reason,
	}
	if !ValidInterruptResumeTransition(&last, PhaseExecute) {
		t.Fatal("execute→execute after process_restart should be allowed")
	}
	if ValidInterruptResumeTransition(&last, PhaseVerify) {
		t.Fatal("execute→verify after process_restart should be rejected")
	}
}
