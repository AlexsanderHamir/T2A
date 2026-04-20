package handler

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// resolvePickupNotBeforeForCreate computes the effective
// `pickup_not_before` for a freshly-created task.
//
// Precedence (locked in the design table — "operator's explicit choice
// wins"):
//
//  1. If the request body carries an explicit `pickup_not_before`
//     string, parse it as RFC3339 UTC and use that. Empty string is
//     rejected on create (use PATCH with "" to clear an existing
//     schedule). Pre-2000 sentinel is rejected to guard against the
//     "0001-01-01T00:00:00Z" zero value sneaking in as "no schedule"
//     via the type-default path.
//
//  2. Otherwise, when the task is being created in StatusReady
//     (default when status is omitted) and
//     settings.AgentPickupDelaySeconds > 0, defer pickup by that
//     many seconds. This preserves the pre-existing global-delay
//     behaviour for callers that don't opt in to per-task scheduling.
//
//  3. Otherwise, return nil (no deferral; the task is eligible
//     immediately when the worker is free).
//
// All times are returned in UTC. Validation errors are wrapped with
// domain.ErrInvalidInput so the standard handler envelope renders a
// 400.
func resolvePickupNotBeforeForCreate(raw *string, status domain.Status, settings domain.AppSettings) (*time.Time, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.resolvePickupNotBeforeForCreate")
	if raw != nil {
		s := strings.TrimSpace(*raw)
		if s == "" {
			return nil, fmt.Errorf("%w: pickup_not_before must not be empty on create (omit the field for no schedule)", domain.ErrInvalidInput)
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("%w: pickup_not_before must be RFC3339 (e.g. 2026-04-19T15:30:00Z): %v", domain.ErrInvalidInput, err)
		}
		if t.Before(pickupNotBeforeMinAllowed) {
			return nil, fmt.Errorf("%w: pickup_not_before must be on or after 2000-01-01T00:00:00Z", domain.ErrInvalidInput)
		}
		out := t.UTC()
		return &out, nil
	}
	if settings.AgentPickupDelaySeconds <= 0 {
		return nil, nil
	}
	st := status
	if st == "" {
		st = domain.StatusReady
	}
	if st != domain.StatusReady {
		return nil, nil
	}
	t := time.Now().UTC().Add(time.Duration(settings.AgentPickupDelaySeconds) * time.Second)
	return &t, nil
}

// pickupNotBeforePatchToStore translates the JSON-layer patch field
// onto the store-layer patch shape. Returns nil when the JSON field
// was omitted (no change requested).
func pickupNotBeforePatchToStore(p patchPickupNotBeforeField) *store.PickupNotBeforePatch {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.pickupNotBeforePatchToStore")
	if !p.Defined {
		return nil
	}
	if p.Clear {
		return &store.PickupNotBeforePatch{Clear: true}
	}
	return &store.PickupNotBeforePatch{At: p.Set}
}
