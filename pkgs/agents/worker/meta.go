package worker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
)

// meta.go owns the payload helpers that produce JSON bytes for the
// store: cycle MetaJSON (buildCycleMeta + sha256Hex) and phase
// details_json (detailsBytes). These are pure functions of their
// inputs; no Worker receiver, no store calls, easy to unit-test in
// isolation.

// buildCycleMeta produces the JSON body written to TaskCycle.MetaJSON.
// The Stage-3 audit contract pins these keys; adding more later is
// allowed but renames require a coordinated migration of the substrate's
// mirror payloads.
//
// Keys (V2):
//   - runner               adapter Name() (e.g. "cursor-cli", "fake")
//   - runner_version       adapter Version() at the time of the cycle
//   - prompt_hash          sha256 of task.InitialPrompt (correlation only)
//   - cursor_model         OPERATOR INTENT: the model string the task
//     was queued with (Task.CursorModel). Empty
//     string means "use the global default", and
//     MUST be persisted as "" not omitted, so the
//     UI/observability code can render the explicit
//     "default" choice.
//   - cursor_model_effective EFFECTIVE: the concrete model identifier
//     the runner resolved for this cycle (calls
//     Runner.EffectiveModel). Empty string is
//     truthful: it means no model was configured
//     anywhere (operator picked the global default
//     AND no DefaultCursorModel is set in
//     app_settings). The Observability runner
//     breakdown panel renders that bucket as
//     "default model".
//
// Keeping both intent and effective lets us answer "the operator
// asked for X but the adapter actually ran Y" without a separate
// audit table.
func buildCycleMeta(r runner.Runner, prompt string, req runner.Request) []byte {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.buildCycleMeta",
		"runner", r.Name())
	out := map[string]string{
		"runner":                 r.Name(),
		"runner_version":         r.Version(),
		"prompt_hash":            sha256Hex(prompt),
		"cursor_model":           req.CursorModel,
		"cursor_model_effective": r.EffectiveModel(req),
	}
	b, err := json.Marshal(out)
	if err != nil {
		slog.Warn("agent worker meta marshal failed", "cmd", workerLogCmd,
			"operation", "agent.worker.buildCycleMeta.err", "err", err)
		return []byte("{}")
	}
	return b
}

// sha256Hex returns the lowercase hex SHA-256 of s. The worker writes
// this into MetaJSON.prompt_hash so the audit trail can correlate runs
// of the same prompt across replays without storing the prompt itself.
func sha256Hex(s string) string {
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.sha256Hex",
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
	slog.Debug("trace", "cmd", workerLogCmd, "operation", "agent.worker.detailsBytes",
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
