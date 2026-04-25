package cursor

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit"
)

func buildDetails(p cursorOutput) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.buildDetails",
		"type", p.Type, "subtype", p.Subtype, "is_error", p.IsError,
		"session_id", p.SessionID, "request_id", p.RequestID,
		"resolved_model", p.ResolvedModel)
	d := struct {
		Type          string          `json:"type,omitempty"`
		Subtype       string          `json:"subtype,omitempty"`
		IsError       bool            `json:"is_error,omitempty"`
		DurationMs    int64           `json:"duration_ms,omitempty"`
		DurationAPIMs int64           `json:"duration_api_ms,omitempty"`
		SessionID     string          `json:"session_id,omitempty"`
		RequestID     string          `json:"request_id,omitempty"`
		Usage         json.RawMessage `json:"usage,omitempty"`
		ResolvedModel string          `json:"resolved_model,omitempty"`
		MissingResult bool            `json:"missing_terminal_result,omitempty"`
	}{
		Type:          p.Type,
		Subtype:       p.Subtype,
		IsError:       p.IsError,
		DurationMs:    p.DurationMs,
		DurationAPIMs: p.DurationAPIMs,
		SessionID:     p.SessionID,
		RequestID:     p.RequestID,
		Usage:         p.Usage,
		ResolvedModel: p.ResolvedModel,
		MissingResult: p.MissingTerminalResult,
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil
	}
	return b
}

func combineStreams(stdout, stderr []byte) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.combineStreams",
		"stdout_bytes", len(stdout), "stderr_bytes", len(stderr))
	return adapterkit.CombineStreams(stdout, stderr)
}

func stderrFirstLineHint(stderr []byte, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.stderrFirstLineHint",
		"stderr_bytes", len(stderr))
	if len(stderr) == 0 {
		return ""
	}
	normalized := strings.ReplaceAll(string(stderr), "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		return clipSummaryRunes(redact(t, homePaths), limits.StderrSummaryHintRunes)
	}
	return ""
}

func timeoutSummary(timeout time.Duration) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.timeoutSummary", "timeout_ns", int64(timeout))
	if timeout > 0 {
		return "cursor: timeout after " + timeout.String()
	}
	return "cursor: cancelled"
}

func execFailedSummary(err error, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.execFailedSummary")
	if err == nil {
		return "cursor: exec failed"
	}
	msg := clipSummaryRunes(redact(strings.TrimSpace(err.Error()), homePaths), limits.StderrSummaryHintRunes)
	if msg == "" {
		return "cursor: exec failed"
	}
	return "cursor: exec failed: " + msg
}

func invalidOutputSummary(err error, homePaths []string) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.invalidOutputSummary")
	if err == nil {
		return "cursor: invalid output"
	}
	msg := clipSummaryRunes(redact(strings.TrimSpace(err.Error()), homePaths), limits.StderrSummaryHintRunes)
	if msg == "" {
		return "cursor: invalid output"
	}
	return "cursor: invalid output: " + msg
}

func clipSummaryRunes(s string, maxRunes int) string {
	return adapterkit.ClipRunes(s, maxRunes)
}

func failureDetails(stage string, err error, stdout, stderr []byte, homePaths []string, extra map[string]any) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.failureDetails",
		"stage", stage, "stdout_bytes", len(stdout), "stderr_bytes", len(stderr))
	out := map[string]any{
		"failure_stage": stage,
	}
	if err != nil {
		out["error"] = redact(err.Error(), homePaths)
	}
	if tail := redactedTail(stdout, homePaths, limits.DiagnosticTailBytes); tail != "" {
		out["stdout_tail"] = tail
	}
	if tail := redactedTail(stderr, homePaths, limits.DiagnosticTailBytes); tail != "" {
		out["stderr_tail"] = tail
	}
	for k, v := range extra {
		out[k] = v
	}
	payload, marshalErr := json.Marshal(out)
	if marshalErr != nil {
		return json.RawMessage(`{"failure_stage":"details_marshal_failed"}`)
	}
	return payload
}

func redactedTail(b []byte, homePaths []string, maxBytes int) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.redactedTail",
		"bytes", len(b), "max_bytes", maxBytes)
	return adapterkit.RedactedTail(b, redactionPolicy(homePaths), maxBytes)
}

func stderrTailDetails(stderr []byte, homePaths []string) json.RawMessage {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.stderrTailDetails",
		"stderr_bytes", len(stderr))
	tail := stderr
	if len(tail) > limits.StderrTailBytes {
		tail = tail[len(tail)-limits.StderrTailBytes:]
		tail = trimLeadingPartialRune(tail)
	}
	redacted := redact(string(tail), homePaths)
	payload, err := json.Marshal(struct {
		StderrTail string `json:"stderr_tail"`
	}{StderrTail: redacted})
	if err != nil {
		return json.RawMessage(`{"stderr_tail":"[redaction failure]"}`)
	}
	return payload
}
