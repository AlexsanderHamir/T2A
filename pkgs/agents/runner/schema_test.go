package runner_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
)

// TestErrCapabilityNotSupported_distinct ensures the new sentinel does
// not collide with existing sentinels.
func TestErrCapabilityNotSupported_distinct(t *testing.T) {
	t.Parallel()

	all := []error{
		runner.ErrTimeout,
		runner.ErrNonZeroExit,
		runner.ErrInvalidOutput,
		runner.ErrCapabilityNotSupported,
	}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) is wrongly matched by %d (%v)", i, a, j, b)
			}
		}
	}
}

// TestConfigSchema_jsonRoundtrip pins the wire format of ConfigSchema.
func TestConfigSchema_jsonRoundtrip(t *testing.T) {
	t.Parallel()

	schema := runner.ConfigSchema{
		Version: 1,
		Fields: []runner.ConfigField{
			{
				Key:      "binary_path",
				Label:    "Binary Path",
				Type:     "string",
				Default:  "/usr/bin/tool",
				Help:     "Where the binary lives.",
				Required: true,
			},
			{
				Key:        "mode",
				Label:      "Operating Mode",
				Type:       "enum",
				Default:    "fast",
				EnumValues: []runner.EnumValue{{Value: "fast", Label: "Fast"}, {Value: "safe", Label: "Safe"}},
			},
			{
				Key:       "api_key",
				Label:     "API Key",
				Type:      "secret",
				Sensitive: true,
			},
		},
	}

	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got runner.ConfigSchema
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Version != schema.Version {
		t.Errorf("version: got %d want %d", got.Version, schema.Version)
	}
	if len(got.Fields) != len(schema.Fields) {
		t.Fatalf("field count: got %d want %d", len(got.Fields), len(schema.Fields))
	}
	for i, f := range got.Fields {
		if f.Key != schema.Fields[i].Key {
			t.Errorf("field %d key: got %q want %q", i, f.Key, schema.Fields[i].Key)
		}
		if f.Type != schema.Fields[i].Type {
			t.Errorf("field %d type: got %q want %q", i, f.Type, schema.Fields[i].Type)
		}
	}
}

// TestConfigSchema_omitemptyFields verifies optional ConfigField fields
// are omitted from JSON when zero.
func TestConfigSchema_omitemptyFields(t *testing.T) {
	t.Parallel()

	field := runner.ConfigField{
		Key:   "name",
		Label: "Name",
		Type:  "string",
	}
	raw, err := json.Marshal(field)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, absent := range []string{"default", "help", "required", "sensitive", "enum_values"} {
		if _, ok := m[absent]; ok {
			t.Errorf("zero-value field %q must be omitted; payload=%s", absent, raw)
		}
	}
	for _, present := range []string{"key", "label", "type"} {
		if _, ok := m[present]; !ok {
			t.Errorf("required field %q must be present; payload=%s", present, raw)
		}
	}
}

// TestModelInfo_jsonRoundtrip pins the wire format of ModelInfo.
func TestModelInfo_jsonRoundtrip(t *testing.T) {
	t.Parallel()

	m := runner.ModelInfo{ID: "claude-4", Label: "Claude 4"}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got runner.ModelInfo
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != m {
		t.Errorf("roundtrip mismatch: got %+v want %+v", got, m)
	}
}

// TestCapabilityInterfaces_fakeImplementsAll is a compile-time +
// runtime check that the fake implements every capability interface.
func TestCapabilityInterfaces_fakeImplementsAll(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()

	if _, ok := any(r).(runner.ConfigSchemaProvider); !ok {
		t.Error("fake does not implement ConfigSchemaProvider")
	}
	if _, ok := any(r).(runner.ConfigValidator); !ok {
		t.Error("fake does not implement ConfigValidator")
	}
	if _, ok := any(r).(runner.Prober); !ok {
		t.Error("fake does not implement Prober")
	}
	if _, ok := any(r).(runner.ModelLister); !ok {
		t.Error("fake does not implement ModelLister")
	}
	if _, ok := any(r).(runner.FailureClassifier); !ok {
		t.Error("fake does not implement FailureClassifier")
	}
	if _, ok := any(r).(runner.MetricsLabeler); !ok {
		t.Error("fake does not implement MetricsLabeler")
	}
	if _, ok := any(r).(runner.CycleMetaProvider); !ok {
		t.Error("fake does not implement CycleMetaProvider")
	}
}

// TestRunnerFake_MetricsLabels checks the fake returns the expected
// Prometheus label shape.
func TestRunnerFake_MetricsLabels(t *testing.T) {
	t.Parallel()

	r := runnerfake.New().WithDefaultModel("opus")
	labels := r.MetricsLabels(runner.Request{CursorModel: "sonnet-4"})
	if labels["model"] != "sonnet-4" {
		t.Errorf("model label: got %q want %q", labels["model"], "sonnet-4")
	}

	labels = r.MetricsLabels(runner.Request{})
	if labels["model"] != "opus" {
		t.Errorf("model label fallback: got %q want %q", labels["model"], "opus")
	}
}

// TestRunnerFake_CycleMeta checks the fake returns the expected audit
// meta shape.
func TestRunnerFake_CycleMeta(t *testing.T) {
	t.Parallel()

	r := runnerfake.New().WithDefaultModel("opus")
	meta := r.CycleMeta(runner.Request{CursorModel: "  sonnet-4  "})

	if meta["cursor_model_intent"] != "sonnet-4" {
		t.Errorf("intent: got %v want %q", meta["cursor_model_intent"], "sonnet-4")
	}
	if meta["cursor_model_effective"] != "sonnet-4" {
		t.Errorf("effective: got %v want %q", meta["cursor_model_effective"], "sonnet-4")
	}

	meta = r.CycleMeta(runner.Request{})
	if meta["cursor_model_intent"] != "" {
		t.Errorf("intent when empty: got %v want %q", meta["cursor_model_intent"], "")
	}
	if meta["cursor_model_effective"] != "opus" {
		t.Errorf("effective fallback: got %v want %q", meta["cursor_model_effective"], "opus")
	}
}

// TestRunnerFake_ConfigSchema checks the fake returns a non-empty schema.
func TestRunnerFake_ConfigSchema(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	schema := r.ConfigSchema()
	if schema.Version != 1 {
		t.Errorf("version: got %d want 1", schema.Version)
	}
	if len(schema.Fields) == 0 {
		t.Error("fields must not be empty")
	}
}

// TestRunnerFake_ValidateConfig checks the fake accepts any blob.
func TestRunnerFake_ValidateConfig(t *testing.T) {
	t.Parallel()

	r := runnerfake.New()
	for _, input := range []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"anything":"goes"}`),
	} {
		if err := r.ValidateConfig(input); err != nil {
			t.Errorf("ValidateConfig(%s): %v", input, err)
		}
	}
}
