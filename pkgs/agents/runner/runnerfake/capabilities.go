package runnerfake

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
)

// ConfigSchema implements runner.ConfigSchemaProvider with a minimal
// test-friendly schema.
func (r *Runner) ConfigSchema() runner.ConfigSchema {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.ConfigSchema")
	return runner.ConfigSchema{
		Version: 1,
		Fields: []runner.ConfigField{
			{
				Key:     "default_model",
				Label:   "Default Model",
				Type:    "string",
				Default: "",
			},
		},
	}
}

// ValidateConfig implements runner.ConfigValidator. Always returns nil
// (accepts any blob).
func (r *Runner) ValidateConfig(blob json.RawMessage) error {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.ValidateConfig")
	return nil
}

// MetricsLabels implements runner.MetricsLabeler. Returns the same
// shape as the cursor adapter: {"model": effectiveModel}.
func (r *Runner) MetricsLabels(req runner.Request) map[string]string {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.MetricsLabels",
		"task_id", req.TaskID)
	m := r.EffectiveModel(req)
	return map[string]string{"model": m}
}

// CycleMeta implements runner.CycleMetaProvider. Returns the same
// audit shape as the cursor adapter for backward-compatible testing.
func (r *Runner) CycleMeta(req runner.Request) map[string]any {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.CycleMeta",
		"task_id", req.TaskID)
	intent := strings.TrimSpace(req.CursorModel)
	effective := r.EffectiveModel(req)
	return map[string]any{
		"cursor_model_intent":    intent,
		"cursor_model_effective": effective,
	}
}

// Probe implements runner.Prober. Returns a static version string from
// the fake's configured version and name.
func (r *Runner) Probe(_ context.Context, binaryPath string, _ time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.Probe",
		"binary", binaryPath)
	r.mu.Lock()
	v := r.version
	r.mu.Unlock()
	return v, binaryPath, nil
}

// ListModels implements runner.ModelLister. Returns a single fake
// model derived from the configured default model.
func (r *Runner) ListModels(_ context.Context, binaryPath string, _ time.Duration) ([]runner.ModelInfo, string, error) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.ListModels",
		"binary", binaryPath)
	r.mu.Lock()
	m := r.defaultModel
	r.mu.Unlock()
	if m == "" {
		return []runner.ModelInfo{{ID: "fake-model", Label: "Fake Model"}}, binaryPath, nil
	}
	return []runner.ModelInfo{{ID: m, Label: m}}, binaryPath, nil
}

// ClassifyFailure implements runner.FailureClassifier. Always returns
// empty strings (no recognized pattern).
func (r *Runner) ClassifyFailure(_ string) (kind, standardizedMsg string) {
	slog.Debug("trace", "cmd", runnerfakeLogCmd, "operation", "runnerfake.Runner.ClassifyFailure")
	return "", ""
}

// Compile-time assertions for all capability interfaces.
var (
	_ runner.ConfigSchemaProvider = (*Runner)(nil)
	_ runner.ConfigValidator      = (*Runner)(nil)
	_ runner.Prober               = (*Runner)(nil)
	_ runner.ModelLister          = (*Runner)(nil)
	_ runner.FailureClassifier    = (*Runner)(nil)
	_ runner.MetricsLabeler       = (*Runner)(nil)
	_ runner.CycleMetaProvider    = (*Runner)(nil)
)
