package orchestration

// TerminationReason is a stable cycle terminate_reason string persisted to the store.
type TerminationReason string

const (
	ReasonVerifyTampered TerminationReason = "verify_tampered"
)

// VerifyEffects lists side effects the harness root applies after DecideVerifyRetry.
type VerifyEffects struct {
	RetryLoop       bool
	TerminalFailure bool
	Tampered        bool
}
