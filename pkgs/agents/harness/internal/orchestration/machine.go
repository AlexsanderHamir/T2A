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

// VerifyDisabled indicates verify is off for this task; the harness runs the
// legacy checklist completion path instead of the adversarial pipeline.
func VerifyDisabled(enabled bool) bool {
	return !enabled
}
