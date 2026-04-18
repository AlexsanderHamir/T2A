package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// docs/API-HTTP.md pins the response invariant for cycle / phase / event
// JSON columns: meta, details, and data are ALWAYS a JSON object (never
// null, never a string/number/array/bool, never the literal "null"). The
// store-side chokepoint (normalizeJSONObject) was tightened in earlier
// sessions, but legacy rows from before that chokepoint can still carry
// non-object literals in meta_json / details_json / data_json. The response
// builder is the last line of defense before bytes hit the wire — if it
// only normalizes len()==0, those legacy rows leak verbatim to clients,
// which then crash on `Object.entries(meta)` (TypeError on null/scalar)
// or render garbage.
//
// These tests pin "non-object literals are normalized to {} on the way out"
// for every response builder that exposes a JSON-object column.

// nonObjectJSONFixtures enumerates every wire-shape that violates the
// "always a JSON object" invariant. Listed inline (not table-driven) so a
// new violation later (eg. an empty string `""`) is added explicitly.
func nonObjectJSONFixtures() map[string][]byte {
	return map[string][]byte{
		"nil":                nil,
		"empty":              []byte{},
		"whitespace":         []byte("   \n\t"),
		"json_null":          []byte("null"),
		"padded_null":        []byte("  null  "),
		"json_string":        []byte(`"hi"`),
		"json_number":        []byte("123"),
		"json_array":         []byte(`[1,2,3]`),
		"json_bool_true":     []byte("true"),
		"json_bool_false":    []byte("false"),
		"malformed_unclosed": []byte(`{"k":`),
	}
}

func assertObjectMessage(t *testing.T, label string, raw json.RawMessage) {
	t.Helper()
	if len(raw) == 0 {
		t.Fatalf("%s: empty json.RawMessage (want a JSON object literal like {})", label)
	}
	if string(raw) == "null" {
		t.Fatalf("%s: emitted JSON null (docs/API-HTTP.md cycle/phase/event invariant: always a JSON object)", label)
	}
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("%s: emitted invalid JSON %q: %v (docs invariant: always a valid JSON object)", label, string(raw), err)
	}
	if _, ok := probe.(map[string]any); !ok {
		t.Fatalf("%s: emitted non-object JSON %q (docs invariant: always a JSON object)", label, string(raw))
	}
}

func TestTaskCycleResponseFromDomain_normalizes_non_object_meta(t *testing.T) {
	for name, raw := range nonObjectJSONFixtures() {
		t.Run(name, func(t *testing.T) {
			c := &domain.TaskCycle{
				ID:          "cyc_1",
				TaskID:      "tsk_1",
				AttemptSeq:  1,
				Status:      domain.CycleStatusRunning,
				StartedAt:   time.Now().UTC(),
				TriggeredBy: domain.ActorUser,
				MetaJSON:    raw,
			}
			resp := taskCycleResponseFromDomain(c)
			assertObjectMessage(t, "taskCycleResponse.Meta", resp.Meta)
		})
	}
}

func TestTaskCyclePhaseResponseFromDomain_normalizes_non_object_details(t *testing.T) {
	for name, raw := range nonObjectJSONFixtures() {
		t.Run(name, func(t *testing.T) {
			p := &domain.TaskCyclePhase{
				ID:          "phs_1",
				CycleID:     "cyc_1",
				Phase:       domain.PhaseDiagnose,
				PhaseSeq:    1,
				Status:      domain.PhaseStatusRunning,
				StartedAt:   time.Now().UTC(),
				DetailsJSON: raw,
			}
			resp := taskCyclePhaseResponseFromDomain(p)
			assertObjectMessage(t, "taskCyclePhaseResponse.Details", resp.Details)
		})
	}
}

func TestTaskCycleDetailFromDomain_normalizes_non_object_meta_and_phase_details(t *testing.T) {
	for name, raw := range nonObjectJSONFixtures() {
		t.Run(name, func(t *testing.T) {
			c := &domain.TaskCycle{
				ID:          "cyc_2",
				TaskID:      "tsk_2",
				AttemptSeq:  1,
				Status:      domain.CycleStatusRunning,
				StartedAt:   time.Now().UTC(),
				TriggeredBy: domain.ActorUser,
				MetaJSON:    raw,
			}
			phases := []domain.TaskCyclePhase{{
				ID:          "phs_2",
				CycleID:     "cyc_2",
				Phase:       domain.PhaseExecute,
				PhaseSeq:    1,
				Status:      domain.PhaseStatusRunning,
				StartedAt:   time.Now().UTC(),
				DetailsJSON: raw,
			}}
			resp := taskCycleDetailFromDomain(c, phases)
			assertObjectMessage(t, "taskCycleDetailResponse.Meta", resp.Meta)
			if len(resp.Phases) != 1 {
				t.Fatalf("expected 1 phase, got %d", len(resp.Phases))
			}
			assertObjectMessage(t, "taskCycleDetailResponse.Phases[0].Details", resp.Phases[0].Details)
		})
	}
}

func TestTaskEventDetailFromDomain_normalizes_non_object_data(t *testing.T) {
	for name, raw := range nonObjectJSONFixtures() {
		t.Run(name, func(t *testing.T) {
			ev := &domain.TaskEvent{
				TaskID: "tsk_3",
				Seq:    1,
				At:     time.Now().UTC(),
				Type:   domain.EventStatusChanged,
				By:     domain.ActorUser,
				Data:   raw,
			}
			resp := taskEventDetailFromDomain(ev, "tsk_3")
			assertObjectMessage(t, "taskEventDetailResponse.Data", resp.Data)
		})
	}
}

func TestTaskEventLines_normalizes_non_object_data(t *testing.T) {
	for name, raw := range nonObjectJSONFixtures() {
		t.Run(name, func(t *testing.T) {
			evs := []domain.TaskEvent{{
				TaskID: "tsk_4",
				Seq:    1,
				At:     time.Now().UTC(),
				Type:   domain.EventStatusChanged,
				By:     domain.ActorUser,
				Data:   raw,
			}}
			lines := taskEventLines(evs)
			if len(lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(lines))
			}
			assertObjectMessage(t, "taskEventLine.Data", lines[0].Data)
		})
	}
}

// TestTaskCycleResponseFromDomain_object_passes_through is the happy-path
// pin: a real JSON object meta survives the response builder unchanged.
// Without it a future "always rewrite to {}" overcorrection would silently
// wipe legitimate runner metadata (eg. the {"runner":"cursor-cli"} doc
// example in models.go:105).
func TestTaskCycleResponseFromDomain_object_passes_through(t *testing.T) {
	c := &domain.TaskCycle{
		ID:          "cyc_ok",
		TaskID:      "tsk_ok",
		AttemptSeq:  1,
		Status:      domain.CycleStatusRunning,
		StartedAt:   time.Now().UTC(),
		TriggeredBy: domain.ActorUser,
		MetaJSON:    []byte(`{"runner":"cursor-cli","prompt_hash":"abc"}`),
	}
	resp := taskCycleResponseFromDomain(c)
	assertObjectMessage(t, "taskCycleResponse.Meta", resp.Meta)
	var got map[string]any
	if err := json.Unmarshal(resp.Meta, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["runner"] != "cursor-cli" {
		t.Fatalf("runner=%v want cursor-cli (object pass-through must preserve fields)", got["runner"])
	}
	if got["prompt_hash"] != "abc" {
		t.Fatalf("prompt_hash=%v want abc (object pass-through must preserve fields)", got["prompt_hash"])
	}
}
