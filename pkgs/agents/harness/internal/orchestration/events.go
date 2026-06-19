package orchestration

// VerifyResult classifies one verify pipeline invocation outcome.
type VerifyResult int

const (
	VerifyResultPass VerifyResult = iota
	VerifyResultFailRetryable
	VerifyResultFailTampered
)
