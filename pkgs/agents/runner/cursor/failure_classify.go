package cursor

import (
	"encoding/json"
	"strings"
)

// Failure kind values stored in runner.Result.Details (JSON) under
// "failure_kind" for stable UI and filtering.
const (
	FailureKindCursorUsageLimit = "cursor_usage_limit"
)

const cursorUsageLimitStdMsg = "Cursor account usage limit reached for the current model. " +
	"Switch to another model in Settings, adjust Spend Limit in the Cursor app, " +
	"or wait until your usage window resets."

const cursorUsageLimitTitle = "Cursor usage limit reached"

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func titleForFailureKind(kind string) string {
	switch kind {
	case FailureKindCursorUsageLimit:
		return cursorUsageLimitTitle
	default:
		return ""
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// classifyCursorFailure inspects combined CLI output (stderr + stdout) and
// returns a stable failure_kind plus a user-facing standardized_message
// when the CLI failure matches a known pattern.
func classifyCursorFailure(combined string) (kind string, standardizedMsg string) {
	lower := strings.ToLower(combined)
	switch {
	case strings.Contains(lower, "usage limit"):
		return FailureKindCursorUsageLimit, cursorUsageLimitStdMsg
	case strings.Contains(lower, "spend limit") && strings.Contains(lower, "continue with this model"):
		return FailureKindCursorUsageLimit, cursorUsageLimitStdMsg
	default:
		return "", ""
	}
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mergeDetailsJSON(base json.RawMessage, extra map[string]any) json.RawMessage {
	if len(extra) == 0 {
		return base
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil || m == nil {
		m = map[string]any{}
	}
	for k, v := range extra {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return base
	}
	return b
}
