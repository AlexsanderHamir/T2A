package verify

import (
	"log/slog"
	"sort"
	"strings"
)

// FormatFailedReason builds terminate-reason for verification exhaustion.
// Prefix "verification_failed" is contract-stable for SPA prefix matching.
func FormatFailedReason(finalVerdicts []Verdict, lockedPasses map[string]Verdict) string {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.verify.FormatFailedReason",
		"verdict_count", len(finalVerdicts), "locked_count", len(lockedPasses))
	failing := make([]string, 0, len(finalVerdicts))
	seen := map[string]struct{}{}
	for _, v := range finalVerdicts {
		if v.Passed {
			continue
		}
		if _, locked := lockedPasses[v.ID]; locked {
			continue
		}
		if _, dup := seen[v.ID]; dup {
			continue
		}
		seen[v.ID] = struct{}{}
		failing = append(failing, v.ID)
	}
	if len(failing) == 0 {
		return failedReasonPrefix
	}
	sort.Strings(failing)
	const maxLen = 256
	const prefix = failedReasonPrefix + ":"
	body := strings.Join(failing, ",")
	full := prefix + body
	if len(full) <= maxLen {
		return full
	}
	const ellipsis = "…"
	budget := maxLen - len(prefix) - len(ellipsis)
	if budget < 0 {
		budget = 0
	}
	if budget > len(body) {
		budget = len(body)
	}
	trimmed := body[:budget]
	if i := strings.LastIndex(trimmed, ","); i > 0 {
		trimmed = trimmed[:i]
	}
	return prefix + trimmed + ellipsis
}
