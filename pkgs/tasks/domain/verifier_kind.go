package domain

// VerifierKind records how a checklist completion was proven.
type VerifierKind string

const (
	VerifierAgentSelf          VerifierKind = "agent_self"
	VerifierVerifyAgent        VerifierKind = "verify_agent"
	VerifierDeterministicCheck VerifierKind = "deterministic_check"
	VerifierHumanOverride      VerifierKind = "human_override"
	VerifierLegacy             VerifierKind = "legacy"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ValidVerifierKind reports whether k is allowed on completion rows.
func ValidVerifierKind(k VerifierKind) bool {
	switch k {
	case VerifierAgentSelf, VerifierVerifyAgent, VerifierDeterministicCheck, VerifierHumanOverride, VerifierLegacy:
		return true
	default:
		return false
	}
}
