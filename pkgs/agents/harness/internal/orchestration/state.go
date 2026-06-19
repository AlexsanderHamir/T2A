package orchestration

// LoopPhase is the high-level position within one execute↔verify iteration.
type LoopPhase int

const (
	LoopPhaseExecute LoopPhase = iota
	LoopPhaseVerify
)

// LoopState captures retry counters the pure machine needs for verify decisions.
// Durable phase rows live in the store; this struct mirrors in-memory scratch only.
type LoopState struct {
	Phase         LoopPhase
	VerifyAttempt int
	MaxRetries    int
	VerifyEnabled bool
}
