package verify

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// LoadSnapshot reads app settings and checklist criteria for verify gating.
func (s *Service) LoadSnapshot(ctx context.Context, taskID string) (Snapshot, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.LoadSnapshot",
		"task_id", taskID)
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	items, err := s.store.ListChecklistForVerify(ctx, taskID)
	if err != nil {
		return Snapshot{}, err
	}
	timeoutSec := settings.VerifyCommandTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = domain.DefaultVerifyCommandTimeoutSeconds
	}
	return Snapshot{
		Enabled:                     len(items) > 0,
		MaxRetries:                  settings.VerifyMaxRetries,
		VerifyCommandTimeoutSeconds: timeoutSec,
		Criteria:                    items,
		VerifyRunner:                s.verifyRunner,
		VerifyModel:                 strings.TrimSpace(settings.VerifyRunnerModel),
	}, nil
}

// CompleteChecklistLegacy marks every checklist item done when verify is disabled.
func (s *Service) CompleteChecklistLegacy(ctx context.Context, taskID string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.CompleteChecklistLegacy", "task_id", taskID)
	items, err := s.store.ListChecklistForSubject(ctx, taskID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Done {
			continue
		}
		if err := s.store.SetChecklistItemDone(ctx, taskID, it.ID, true, domain.ActorAgent); err != nil {
			return err
		}
	}
	return nil
}

// ApplyVerifiedCompletions writes checklist completions for passed verdicts.
func (s *Service) ApplyVerifiedCompletions(ctx context.Context, taskID, cycleID string, verdicts []Verdict) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.ApplyVerifiedCompletions",
		"task_id", taskID, "cycle_id", cycleID, "verdict_count", len(verdicts))
	for _, v := range verdicts {
		if !v.Passed {
			continue
		}
		err := s.store.SetChecklistItemDoneWithEvidence(ctx, taskID, v.ID, v.Evidence, v.Verifier, v.Reasoning, cycleID, domain.ActorAgent)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}
	return nil
}
