package cycles

import (
	"strings"
	"unicode/utf8"
)

// MaxFailureSurfaceRunes caps operator-facing failure text on cycle_failed
// mirrors and /tasks/stats recent_failures so payloads stay bounded.
const MaxFailureSurfaceRunes = 800

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// FailureSurfaceMessage returns the best human-facing explanation for a
// failed terminal cycle, preferring execute-phase classification
// (standardized_message, summary, failure_kind) over the cycle mirror
// reason code (e.g. runner_non_zero_exit).
//
// When hasPhase is false, the function returns "" (caller keeps the
// cycle-only projection). When hasPhase is true but no richer fields
// exist, cycleReason is returned so Observability still has a stable
// string.
func FailureSurfaceMessage(hasPhase bool, cycleReason, phaseSummary string, phaseDetails map[string]any) string {
	if !hasPhase {
		return ""
	}
	if msg := standardizedMessageFromDetails(phaseDetails); msg != "" {
		return truncateReasonRunes(msg, MaxFailureSurfaceRunes)
	}
	if s := strings.TrimSpace(phaseSummary); s != "" {
		return truncateReasonRunes(s, MaxFailureSurfaceRunes)
	}
	if fk := failureKindFromDetails(phaseDetails); fk != "" {
		if h := humanizeFailureKind(fk); h != "" {
			return h
		}
		return fk
	}
	if strings.TrimSpace(cycleReason) == "" {
		return ""
	}
	return cycleReason
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func standardizedMessageFromDetails(d map[string]any) string {
	if d == nil {
		return ""
	}
	v, ok := d["standardized_message"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func failureKindFromDetails(d map[string]any) string {
	if d == nil {
		return ""
	}
	v, ok := d["failure_kind"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func humanizeFailureKind(kind string) string {
	switch kind {
	case "cursor_usage_limit":
		return "Cursor usage limit reached"
	default:
		return ""
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func truncateReasonRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}
