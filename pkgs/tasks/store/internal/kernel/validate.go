package kernel

import (
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ValidStatus reports whether s is a writable domain.Status enum.
func ValidStatus(s domain.Status) bool {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidStatus")
	switch s {
	case domain.StatusReady, domain.StatusRunning, domain.StatusBlocked, domain.StatusReview, domain.StatusDone, domain.StatusFailed, domain.StatusOnHold:
		return true
	default:
		return false
	}
}

// ValidClientWritableStatus reports whether a client may set s on create or PATCH.
func ValidClientWritableStatus(s domain.Status) bool {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidClientWritableStatus")
	return ValidStatus(s)
}

// ValidPriority reports whether p is a writable domain.Priority enum.
func ValidPriority(p domain.Priority) bool {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidPriority")
	switch p {
	case domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical:
		return true
	default:
		return false
	}
}

// ValidateActor returns domain.ErrInvalidInput when a is not a known actor enum.
func ValidateActor(a domain.Actor) error {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidateActor")
	switch a {
	case domain.ActorUser, domain.ActorAgent:
		return nil
	default:
		return fmt.Errorf("%w: actor", domain.ErrInvalidInput)
	}
}

// ValidPhase reports whether p is a known domain.Phase enum.
func ValidPhase(p domain.Phase) bool {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidPhase")
	switch p {
	case domain.PhaseExecute, domain.PhaseVerify:
		return true
	default:
		return false
	}
}

// ValidTerminalCycleStatus reports whether s is a terminal CycleStatus.
func ValidTerminalCycleStatus(s domain.CycleStatus) bool {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidTerminalCycleStatus")
	return domain.TerminalCycleStatus(s)
}

// ValidTerminalPhaseStatus reports whether s is a terminal PhaseStatus.
func ValidTerminalPhaseStatus(s domain.PhaseStatus) bool {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.ValidTerminalPhaseStatus")
	return domain.TerminalPhaseStatus(s)
}
