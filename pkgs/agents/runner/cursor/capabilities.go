package cursor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
)

// ---------------------------------------------------------------------------
// ConfigSchemaProvider + ConfigValidator
// ---------------------------------------------------------------------------

// ConfigSchema implements runner.ConfigSchemaProvider.
func (a *Adapter) ConfigSchema() runner.ConfigSchema {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.ConfigSchema")
	return runner.ConfigSchema{
		Version: 1,
		Fields: []runner.ConfigField{
			{
				Key:      "binary_path",
				Label:    "Binary Path",
				Type:     "string",
				Default:  defaults.BinaryPath,
				Help:     "Absolute path or command name for the cursor-agent executable.",
				Required: false,
			},
			{
				Key:      "default_model",
				Label:    "Default Model",
				Type:     "string",
				Default:  "",
				Help:     "Model identifier to use when the task does not specify one.",
				Required: false,
			},
		},
	}
}

// cursorConfigBlob is the JSON shape stored in runner_configs["cursor"].
type cursorConfigBlob struct {
	BinaryPath   string `json:"binary_path,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
}

// ValidateConfig implements runner.ConfigValidator. It accepts any
// combination of the known keys and rejects unknown top-level keys.
func (a *Adapter) ValidateConfig(blob json.RawMessage) error {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.ValidateConfig")
	if len(blob) == 0 || string(blob) == "null" || string(blob) == "{}" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(blob, &m); err != nil {
		return fmt.Errorf("cursor config: invalid JSON: %w", err)
	}
	allowed := map[string]bool{"binary_path": true, "default_model": true}
	for k := range m {
		if !allowed[k] {
			return fmt.Errorf("cursor config: unknown key %q", k)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Prober
// ---------------------------------------------------------------------------

// Probe implements runner.Prober. It delegates to the package-level Probe
// function and ResolveBinaryPath, using the adapter's probe function or
// the production default.
func (a *Adapter) Probe(ctx context.Context, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Probe",
		"binary", binaryPath)
	resolved := ResolveBinaryPath(binaryPath)
	if resolved == "" {
		resolved = binaryPath
	}
	ver, probeErr := Probe(ctx, binaryPath, timeout, nil)
	return ver, resolved, probeErr
}

// ---------------------------------------------------------------------------
// ModelLister
// ---------------------------------------------------------------------------

// ListModels implements runner.ModelLister. Returns runner.ModelInfo
// (the generic type) by delegating to the package-level ListModels.
func (a *Adapter) ListModels(ctx context.Context, binaryPath string, timeout time.Duration) ([]runner.ModelInfo, string, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.ListModels",
		"binary", binaryPath)
	cursorModels, resolved, err := ListModels(ctx, binaryPath, timeout, nil)
	if err != nil {
		return nil, resolved, err
	}
	out := make([]runner.ModelInfo, len(cursorModels))
	for i, m := range cursorModels {
		out[i] = runner.ModelInfo{ID: m.ID, Label: m.Label}
	}
	return out, resolved, nil
}

// ---------------------------------------------------------------------------
// FailureClassifier
// ---------------------------------------------------------------------------

// ClassifyFailure implements runner.FailureClassifier by delegating to
// the package-level classifyCursorFailure.
func (a *Adapter) ClassifyFailure(combined string) (kind, standardizedMsg string) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.ClassifyFailure")
	return classifyCursorFailure(combined)
}

// ---------------------------------------------------------------------------
// MetricsLabeler
// ---------------------------------------------------------------------------

// MetricsLabels implements runner.MetricsLabeler. Returns the label set
// the worker uses for Prometheus metrics. The "model" key preserves
// backward-compatible series shape.
func (a *Adapter) MetricsLabels(req runner.Request) map[string]string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.MetricsLabels",
		"task_id", req.TaskID)
	m := a.EffectiveModel(req)
	return map[string]string{"model": m}
}

// ---------------------------------------------------------------------------
// CycleMetaProvider
// ---------------------------------------------------------------------------

// CycleMeta implements runner.CycleMetaProvider. Returns the key-value
// pairs the worker writes into TaskCycle.MetaJSON, preserving the
// existing audit-trail shape.
func (a *Adapter) CycleMeta(req runner.Request) map[string]any {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.CycleMeta",
		"task_id", req.TaskID)
	intent := strings.TrimSpace(req.CursorModel)
	effective := a.EffectiveModel(req)
	return map[string]any{
		"cursor_model_intent":    intent,
		"cursor_model_effective": effective,
	}
}

// ---------------------------------------------------------------------------
// Compile-time assertions for all capability interfaces.
// ---------------------------------------------------------------------------

var (
	_ runner.ConfigSchemaProvider = (*Adapter)(nil)
	_ runner.ConfigValidator      = (*Adapter)(nil)
	_ runner.Prober               = (*Adapter)(nil)
	_ runner.ModelLister          = (*Adapter)(nil)
	_ runner.FailureClassifier    = (*Adapter)(nil)
	_ runner.MetricsLabeler       = (*Adapter)(nil)
	_ runner.CycleMetaProvider    = (*Adapter)(nil)
)
