package kernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// NormalizeJSONObject pins the on-disk shape for JSON object payloads
// across the store: drafts (task_drafts.payload_json), cycles
// (task_cycles.metadata_json), and any other store column documented
// as "always a JSON object". An empty / whitespace-only / "null" input
// collapses to "{}" so downstream readers never observe SQL NULL or
// the JSON literal null. Anything else must be a syntactically valid
// JSON object; a string / number / array / bool / malformed input
// returns domain.ErrInvalidInput so handlers surface a 400.
//
// The returned bytes are always whitespace-trimmed at the document
// boundaries: the empty/null branch returns the canonical "{}", and
// the validated-object branch returns the trimmed slice rather than
// the caller's original bytes. Without this, valid-object inputs
// padded with leading or trailing whitespace (e.g. "  {\"a\":1}\n")
// were persisted with the surrounding whitespace intact, while the
// empty branch always emitted exactly "{}" — leaving the on-disk
// shape inconsistent (two callers writing the same logical payload
// could end up with byte-different rows depending on which branch
// fired) and inflating column size with no semantic value. Trimming
// at the boundary keeps the dual-write audit mirror, the response
// chokepoint (handler.normalizeJSONObjectForResponse), and any
// future byte-equality assertion in sync. Interior whitespace inside
// the JSON document is preserved by encoding/json's tokenizer so
// pretty-printed payloads still round-trip with their formatting
// intact.
//
// field is the human-readable name of the offending column; it is
// only used to format the wrapped error.
func NormalizeJSONObject(b []byte, field string) ([]byte, error) {
	slog.Debug("trace", "cmd", LogCmd, "operation", "tasks.store.kernel.NormalizeJSONObject")
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []byte("{}"), nil
	}
	var probe any
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return nil, fmt.Errorf("%w: %s must be a JSON object", domain.ErrInvalidInput, field)
	}
	if _, ok := probe.(map[string]any); !ok {
		return nil, fmt.Errorf("%w: %s must be a JSON object", domain.ErrInvalidInput, field)
	}
	return trimmed, nil
}
