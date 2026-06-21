package harness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// meta.go owns the payload helpers that produce JSON bytes for the
// store: cycle MetaJSON (buildCycleMeta + sha256Hex) and phase
// details_json (detailsBytes). These are pure functions of their
// inputs; no Worker receiver, no store calls, easy to unit-test in
// isolation.

// buildCycleMeta produces the JSON body written to TaskCycle.MetaJSON.
// Common keys (runner, runner_version, prompt_hash) are always present.
// Runner-specific keys come from the runner.CycleMetaProvider capability
// interface — adapters contribute their own metadata (model intent,
// effective model, etc.) without the worker knowing about adapter
// internals.
//
// When the runner does not implement CycleMetaProvider, the common keys
// are emitted alone.
func buildCycleMeta(r runner.Runner, prompt string, req runner.Request) []byte {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.buildCycleMeta",
		"runner", r.Name())
	out := map[string]any{
		"runner":         r.Name(),
		"runner_version": r.Version(),
		"prompt_hash":    sha256Hex(prompt),
	}
	if cmp, ok := r.(runner.CycleMetaProvider); ok {
		for k, v := range cmp.CycleMeta(req) {
			out[k] = v
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		slog.Warn("agent harness meta marshal failed", "cmd", harnessLogCmd,
			"operation", "agent.harness.buildCycleMeta.err", "err", err)
		return []byte("{}")
	}
	return b
}

// mergeCycleMetaBytes overlays extra keys onto cycle MetaJSON bytes.
// retryModeFromCycleMeta reads operator retry mode stamped on cycle MetaJSON.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func retryModeFromCycleMeta(cycle *domain.TaskCycle) domain.RetryMode {
	if cycle == nil || len(cycle.MetaJSON) == 0 {
		return ""
	}
	var meta struct {
		RetryMode string `json:"retry_mode"`
	}
	if err := json.Unmarshal(cycle.MetaJSON, &meta); err != nil {
		return ""
	}
	return domain.RetryMode(strings.TrimSpace(meta.RetryMode))
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mergeCycleMetaBytes(base []byte, extra map[string]any) []byte {
	if len(extra) == 0 {
		return base
	}
	out := map[string]any{}
	if len(base) > 0 {
		_ = json.Unmarshal(base, &out)
	}
	for k, v := range extra {
		out[k] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return base
	}
	return b
}

// sha256Hex returns the lowercase hex SHA-256 of s. The worker writes
// this into MetaJSON.prompt_hash so the audit trail can correlate runs
// of the same prompt across replays without storing the prompt itself.
func sha256Hex(s string) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.sha256Hex",
		"len", len(s))
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// detailsBytes converts a runner.Result's free-form Details into the
// JSON object the store expects. The store's kernel.NormalizeJSONObject
// chokepoint requires details_json to be a JSON object on every write
// (sessions 1+2 of .agent/bug-hunting-agent.log) — non-object payloads
// surface as domain.ErrInvalidInput. runner.Result.Details is typed
// json.RawMessage and adapters like cursor forward whatever the CLI
// emitted, so the worker is the chokepoint that has to coerce. (When
// CompletePhase fails for any other reason, processOne now falls
// through to bestEffortTerminate so the cycle row never lingers in
// `running`; the orphan-sweep at startup is the last-resort safety
// net.)
//
// Rules (matching the store-side normalize:
//
//   - nil / empty / whitespace / "null" -> "{}"
//   - valid JSON object -> pass through verbatim
//   - valid JSON non-object (string, number, array, bool) -> wrapped
//     as {"value": <raw>} so the original parsed value survives in the
//     audit trail
//   - malformed JSON -> wrapped as {"raw": "<original-bytes-as-string>"}
//     so the diagnostic bytes are preserved (still parseable JSON)
func detailsBytes(r runner.Result) []byte {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.detailsBytes",
		"len", len(r.Details))
	trimmed := bytes.TrimSpace(r.Details)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []byte("{}")
	}
	if trimmed[0] == '{' && json.Valid(trimmed) {
		return r.Details
	}
	if json.Valid(trimmed) {
		out := make([]byte, 0, len(trimmed)+12)
		out = append(out, []byte(`{"value":`)...)
		out = append(out, trimmed...)
		out = append(out, '}')
		return out
	}
	encoded, err := json.Marshal(string(r.Details))
	if err != nil {
		return []byte("{}")
	}
	out := make([]byte, 0, len(encoded)+10)
	out = append(out, []byte(`{"raw":`)...)
	out = append(out, encoded...)
	out = append(out, '}')
	return out
}
