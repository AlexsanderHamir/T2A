package orchestration

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

// DecideVerifyDisabledLegacy maps legacy checklist completion outcome to effects.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func DecideVerifyDisabledLegacy(checklistErr error) VerifyEffects {
	if checklistErr != nil {
		return VerifyEffects{TerminalFailure: true}
	}
	return VerifyEffects{}
}

// DecideFinalizeSuccess maps completion ledger outcome to terminal cycle/task status.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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
