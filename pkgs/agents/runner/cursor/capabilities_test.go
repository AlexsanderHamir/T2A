package cursor_test

import (
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
)

// TestCursorAdapter_ConfigSchema_fields validates the cursor adapter
// exposes the expected schema fields.
func TestCursorAdapter_ConfigSchema_fields(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{})
	schema := a.ConfigSchema()
	if schema.Version != 1 {
		t.Errorf("version: got %d want 1", schema.Version)
	}

	keys := map[string]bool{}
	for _, f := range schema.Fields {
		keys[f.Key] = true
	}
	for _, want := range []string{"binary_path", "default_model"} {
		if !keys[want] {
			t.Errorf("missing schema field %q", want)
		}
	}
}

// TestCursorAdapter_ConfigSchema_jsonRoundtrip ensures the schema
// survives serialization for the SPA.
func TestCursorAdapter_ConfigSchema_jsonRoundtrip(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{})
	schema := a.ConfigSchema()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got runner.ConfigSchema
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Version != schema.Version {
		t.Errorf("version mismatch after roundtrip")
	}
	if len(got.Fields) != len(schema.Fields) {
		t.Errorf("field count mismatch: got %d want %d", len(got.Fields), len(schema.Fields))
	}
}

// TestCursorAdapter_ValidateConfig_knownKeys checks that known keys are
// accepted and unknown keys are rejected.
func TestCursorAdapter_ValidateConfig_knownKeys(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{})

	valid := []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`null`),
		json.RawMessage(`{"binary_path":"/usr/bin/cursor"}`),
		json.RawMessage(`{"default_model":"opus"}`),
		json.RawMessage(`{"binary_path":"cursor","default_model":"sonnet"}`),
	}
	for _, v := range valid {
		if err := a.ValidateConfig(v); err != nil {
			t.Errorf("ValidateConfig(%s): unexpected error: %v", v, err)
		}
	}

	invalid := []json.RawMessage{
		json.RawMessage(`{"unknown_key":"val"}`),
		json.RawMessage(`not json`),
	}
	for _, v := range invalid {
		if err := a.ValidateConfig(v); err == nil {
			t.Errorf("ValidateConfig(%s): expected error for invalid input", v)
		}
	}
}

// TestCursorAdapter_MetricsLabels checks backward-compatible label shape.
func TestCursorAdapter_MetricsLabels(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{DefaultCursorModel: "opus"})

	labels := a.MetricsLabels(runner.Request{CursorModel: "sonnet-4"})
	if labels["model"] != "sonnet-4" {
		t.Errorf("model label: got %q want %q", labels["model"], "sonnet-4")
	}

	labels = a.MetricsLabels(runner.Request{})
	if labels["model"] != "opus" {
		t.Errorf("model label fallback: got %q want %q", labels["model"], "opus")
	}
}

// TestCursorAdapter_CycleMeta checks backward-compatible audit shape.
func TestCursorAdapter_CycleMeta(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{DefaultCursorModel: "opus"})

	meta := a.CycleMeta(runner.Request{CursorModel: "sonnet-4"})
	if meta["cursor_model_intent"] != "sonnet-4" {
		t.Errorf("intent: got %v want %q", meta["cursor_model_intent"], "sonnet-4")
	}
	if meta["cursor_model_effective"] != "sonnet-4" {
		t.Errorf("effective: got %v want %q", meta["cursor_model_effective"], "sonnet-4")
	}

	meta = a.CycleMeta(runner.Request{})
	if meta["cursor_model_intent"] != "" {
		t.Errorf("intent empty request: got %v want %q", meta["cursor_model_intent"], "")
	}
	if meta["cursor_model_effective"] != "opus" {
		t.Errorf("effective fallback: got %v want %q", meta["cursor_model_effective"], "opus")
	}
}

// TestCursorAdapter_ClassifyFailure checks known failure patterns.
func TestCursorAdapter_ClassifyFailure(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{})

	kind, _ := a.ClassifyFailure("error: Usage Limit exceeded for this model")
	if kind != "cursor_usage_limit" {
		t.Errorf("usage limit: got kind=%q want %q", kind, "cursor_usage_limit")
	}

	kind, _ = a.ClassifyFailure("everything is fine")
	if kind != "" {
		t.Errorf("no match: got kind=%q want empty", kind)
	}
}

// TestCursorAdapter_implementsAllCapabilities verifies compile-time
// + runtime that the cursor adapter satisfies every capability
// interface.
func TestCursorAdapter_implementsAllCapabilities(t *testing.T) {
	t.Parallel()

	a := cursor.New(cursor.Options{})
	r := runner.Runner(a)

	if _, ok := r.(runner.ConfigSchemaProvider); !ok {
		t.Error("cursor adapter does not implement ConfigSchemaProvider")
	}
	if _, ok := r.(runner.ConfigValidator); !ok {
		t.Error("cursor adapter does not implement ConfigValidator")
	}
	if _, ok := r.(runner.Prober); !ok {
		t.Error("cursor adapter does not implement Prober")
	}
	if _, ok := r.(runner.ModelLister); !ok {
		t.Error("cursor adapter does not implement ModelLister")
	}
	if _, ok := r.(runner.FailureClassifier); !ok {
		t.Error("cursor adapter does not implement FailureClassifier")
	}
	if _, ok := r.(runner.MetricsLabeler); !ok {
		t.Error("cursor adapter does not implement MetricsLabeler")
	}
	if _, ok := r.(runner.CycleMetaProvider); !ok {
		t.Error("cursor adapter does not implement CycleMetaProvider")
	}
}
