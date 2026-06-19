package tasks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

const (
	gateActionRelease   = "release"
	gateActionHold      = "hold"
	gateActionClearHold = "clear_hold"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ApplyTaskGateAction mutates a task's gate per operator action and persists.
func ApplyTaskGateAction(ctx context.Context, db *gorm.DB, taskID, action string, by domain.Actor) (*domain.Task, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	if action == "" {
		return nil, fmt.Errorf("%w: action required", domain.ErrInvalidInput)
	}
	t, err := Get(ctx, db, taskID)
	if err != nil {
		return nil, err
	}
	if t.Gate == nil {
		return nil, fmt.Errorf("%w: task has no gate", domain.ErrInvalidInput)
	}
	g := *t.Gate
	now := time.Now().UTC()
	switch action {
	case gateActionRelease:
		g.Status = domain.GateStatusReleased
		g.Hold = false
	case gateActionHold:
		if g.Status != domain.GateStatusPendingRelease {
			return nil, fmt.Errorf("%w: hold only applies while gate is pending_release", domain.ErrInvalidInput)
		}
		g.Hold = true
	case gateActionClearHold:
		g.Hold = false
		if g.Status == domain.GateStatusPendingRelease &&
			g.PendingReleaseDeadlineUTC != nil && !now.Before(*g.PendingReleaseDeadlineUTC) {
			g.Status = domain.GateStatusReleased
		}
	default:
		return nil, fmt.Errorf("%w: invalid gate action", domain.ErrInvalidInput)
	}
	gate := g
	gatePtr := &gate
	updated, _, err := Update(ctx, db, taskID, UpdateInput{Gate: &gatePtr}, by)
	if err != nil {
		return nil, err
	}
	return updated, nil
}
