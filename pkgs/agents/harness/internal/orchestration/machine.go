package orchestration

// DecideVerifyRetry maps verify pipeline outcome + retry budget to effects.
// Attempt is the current verifyAttempt before any increment; the harness root
// increments verifyAttempt when RetryLoop is true.
func DecideVerifyRetry(attempt, maxRetries int, result VerifyResult) VerifyEffects {
	switch result {
	case VerifyResultPass:
		return VerifyEffects{}
	case VerifyResultFailTampered:
		return VerifyEffects{TerminalFailure: true, Tampered: true}
	case VerifyResultFailRetryable:
		if attempt < maxRetries {
			return VerifyEffects{RetryLoop: true}
		}
		return VerifyEffects{TerminalFailure: true}
	default:
		return VerifyEffects{TerminalFailure: true}
	}
}

// DecideVerifyRetryWithValidity extends DecideVerifyRetry with in-cycle
// verify-only retry when execute artifacts remain valid (ADR-0028).
func DecideVerifyRetryWithValidity(attempt, maxRetries int, result VerifyResult, executeStillValid bool) VerifyEffects {
	effects := DecideVerifyRetry(attempt, maxRetries, result)
	if effects.RetryLoop && executeStillValid {
		effects.SkipNextExecute = true
	}
	return effects
}

// VerifyDisabled indicates verify is off for this task; the harness runs the
// legacy checklist completion path instead of the adversarial pipeline.
func VerifyDisabled(enabled bool) bool {
	return !enabled
}
