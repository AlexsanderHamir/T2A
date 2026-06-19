package orchestration

import "github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"

// DecideVerifyDisabledLegacy maps legacy checklist completion outcome to effects.
func DecideVerifyDisabledLegacy(checklistErr error) VerifyEffects {
	if checklistErr != nil {
		return VerifyEffects{TerminalFailure: true}
	}
	return VerifyEffects{}
}

// DecideFinalizeSuccess maps completion ledger outcome to terminal cycle/task status.
func DecideFinalizeSuccess(completionErr error) FinalizeEffects {
	if completionErr != nil {
		return FinalizeEffects{
			CycleStatus: domain.CycleStatusFailed,
			TaskStatus:  domain.StatusFailed,
			Reason:      ReasonChecklistCompletionFailed,
		}
	}
	return FinalizeEffects{
		CycleStatus: domain.CycleStatusSucceeded,
		TaskStatus:  domain.StatusDone,
	}
}
