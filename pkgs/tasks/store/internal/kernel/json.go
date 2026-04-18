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
	return b, nil
}
