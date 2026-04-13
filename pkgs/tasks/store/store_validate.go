package store

import (
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func validStatus(s domain.Status) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validStatus")
	switch s {
	case domain.StatusReady, domain.StatusRunning, domain.StatusBlocked, domain.StatusReview, domain.StatusDone, domain.StatusFailed:
		return true
	default:
		return false
	}
}

func validPriority(p domain.Priority) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validPriority")
	switch p {
	case domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical:
		return true
	default:
		return false
	}
}

func validTaskType(t domain.TaskType) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validTaskType")
	switch t {
	case domain.TaskTypeGeneral, domain.TaskTypeBugFix, domain.TaskTypeFeature, domain.TaskTypeRefactor, domain.TaskTypeDocs:
		return true
	default:
		return false
	}
}

func validateActor(a domain.Actor) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.validateActor")
	switch a {
	case domain.ActorUser, domain.ActorAgent:
		return nil
	default:
		return fmt.Errorf("%w: actor", domain.ErrInvalidInput)
	}
}
