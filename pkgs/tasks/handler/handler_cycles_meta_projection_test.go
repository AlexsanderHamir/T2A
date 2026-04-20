package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

// TestProjectCycleMeta_extractsTypedFields pins the Phase 1b contract
// for the typed `cycle_meta` projection. Empty strings are SEMANTIC
// values (operator-default vs adapter-default vs no-value-anywhere)
// and MUST be preserved end-to-end so the SPA can render the
// "default model" bucket without inventing one.
func TestProjectCycleMeta_extractsTypedFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want cycleMetaProjection
	}{
		{
			name: "all_fields_populated",
			raw: `{"runner":"cursor-cli","runner_version":"0.42.0",` +
				`"cursor_model":"opus","cursor_model_effective":"opus",` +
				`"prompt_hash":"abc123"}`,
			want: cycleMetaProjection{
				Runner:               "cursor-cli",
				RunnerVersion:        "0.42.0",
				CursorModel:          "opus",
				CursorModelEffective: "opus",
				PromptHash:           "abc123",
			},
		},
		{
			name: "intent_empty_effective_resolved",
			raw: `{"runner":"cursor-cli","runner_version":"0.42.0",` +
				`"cursor_model":"","cursor_model_effective":"opus",` +
				`"prompt_hash":"abc"}`,
			want: cycleMetaProjection{
				Runner:               "cursor-cli",
				RunnerVersion:        "0.42.0",
				CursorModel:          "",
				CursorModelEffective: "opus",
				PromptHash:           "abc",
			},
		},
		{
			name: "pre_feature_cycle_no_model_keys",
			raw:  `{"runner":"cursor-cli","runner_version":"0.42.0","prompt_hash":"abc"}`,
			want: cycleMetaProjection{
				Runner:        "cursor-cli",
				RunnerVersion: "0.42.0",
				PromptHash:    "abc",
			},
		},
		{
			name: "completely_empty_object",
			raw:  `{}`,
			want: cycleMetaProjection{},
		},
		{
			name: "unknown_keys_ignored",
			raw: `{"runner":"cursor-cli","extra_future_key":"ignored",` +
				`"cursor_model":"opus","prompt_hash":"abc"}`,
			want: cycleMetaProjection{
				Runner:      "cursor-cli",
				CursorModel: "opus",
				PromptHash:  "abc",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := projectCycleMeta(json.RawMessage(tc.raw))
			if got != tc.want {
				t.Errorf("projectCycleMeta(%s) = %+v want %+v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestProjectCycleMeta_emptyInputReturnsZero guards the nil/empty
// edge case. normalizeJSONObjectForResponse always returns at least
// `{}`, but if a future caller drops that contract we want a quiet
// zero value (logged) rather than a JSON decode error escaping into
// the cycles handler.
func TestProjectCycleMeta_emptyInputReturnsZero(t *testing.T) {
	t.Parallel()

	for _, in := range []json.RawMessage{nil, json.RawMessage("")} {
		got := projectCycleMeta(in)
		if got != (cycleMetaProjection{}) {
			t.Errorf("projectCycleMeta(%q) = %+v want zero value", in, got)
		}
	}
}

// TestTaskCycleResponseFromDomain_includesCycleMeta verifies the
// projection survives the full domain.TaskCycle -> JSON response
// hop. The Phase 1b SPA contract requires `cycle_meta` to be
// present on every cycle row even when MetaJSON is missing the
// V2 keys.
func TestTaskCycleResponseFromDomain_includesCycleMeta(t *testing.T) {
	t.Parallel()

	cycle := &domain.TaskCycle{
		ID:         "11111111-1111-4111-8111-111111111111",
		TaskID:     "22222222-2222-4222-8222-222222222222",
		AttemptSeq: 1,
		Status:     domain.CycleStatusSucceeded,
		MetaJSON: datatypes.JSON([]byte(
			`{"runner":"cursor-cli","runner_version":"0.42.0",` +
				`"cursor_model":"","cursor_model_effective":"opus",` +
				`"prompt_hash":"deadbeef"}`,
		)),
	}

	resp := taskCycleResponseFromDomain(cycle)
	if resp.CycleMeta.Runner != "cursor-cli" {
		t.Errorf("runner = %q want cursor-cli (cycle_meta=%+v)", resp.CycleMeta.Runner, resp.CycleMeta)
	}
	if resp.CycleMeta.CursorModelEffective != "opus" {
		t.Errorf("cursor_model_effective = %q want opus", resp.CycleMeta.CursorModelEffective)
	}
	if resp.CycleMeta.CursorModel != "" {
		t.Errorf("cursor_model = %q want empty (operator default)", resp.CycleMeta.CursorModel)
	}

	// The raw `meta` envelope MUST keep the original bytes
	// untouched (forwards-compat: future MetaJSON keys flow through
	// without a typed projection bump).
	if !strings.Contains(string(resp.Meta), `"prompt_hash":"deadbeef"`) {
		t.Errorf("raw meta lost original keys: %s", resp.Meta)
	}
}
