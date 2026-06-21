// Package claudecode is a scaffold adapter for the Claude Code CLI
// runner. It implements runner.Runner and the full capability interface
// set so the generic registration path is exercised end-to-end.
//
// Status: SCAFFOLD. The Run method returns ErrTimeout unconditionally;
// the adapter is not yet wired to a real CLI binary. Use this package
// as a template when adding a production Claude Code integration.
package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

const adapterLogCmd = "claudecode"

// Options configures the Claude Code adapter.
type Options struct {
	BinaryPath   string
	Version      string
	DefaultModel string
}

// Adapter implements runner.Runner for the Claude Code CLI.
type Adapter struct {
	opts Options
}

// New creates a new Claude Code adapter.
func New(opts Options) *Adapter {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.New")
	return &Adapter{opts: opts}
}

func (a *Adapter) Name() string {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.Name")
	return "claude-code"
}

func (a *Adapter) Version() string {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.Version")
	return a.opts.Version
}

// EffectiveModel returns the model that would be used for a given
// request: the request-level model wins, then the adapter default,
// then empty string (CLI default).
func (a *Adapter) EffectiveModel(req runner.Request) string {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.EffectiveModel")
	if m := strings.TrimSpace(req.CursorModel); m != "" {
		return m
	}
	return strings.TrimSpace(a.opts.DefaultModel)
}

// Run is a scaffold: it returns ErrTimeout so the worker marks the
// cycle failed rather than hanging indefinitely. Replace with a real
// CLI invocation when the Claude Code integration is ready.
func (a *Adapter) Run(_ context.Context, req runner.Request) (runner.Result, error) {
	slog.Warn("claude-code adapter is a scaffold; Run always fails",
		"cmd", adapterLogCmd, "task_id", req.TaskID)
	return runner.NewResult(
		domain.PhaseStatusFailed,
		"claude-code adapter is not yet implemented",
		nil,
		"",
	), runner.ErrTimeout
}

// ---------------------------------------------------------------------------
// ConfigSchemaProvider + ConfigValidator
// ---------------------------------------------------------------------------

func (a *Adapter) ConfigSchema() runner.ConfigSchema {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.ConfigSchema")
	return runner.ConfigSchema{
		Version: 1,
		Fields: []runner.ConfigField{
			{
				Key:      "binary_path",
				Label:    "Binary Path",
				Type:     "string",
				Default:  "claude",
				Help:     "Absolute path or command name for the claude CLI executable.",
				Required: false,
			},
			{
				Key:      "default_model",
				Label:    "Default Model",
				Type:     "string",
				Default:  "",
				Help:     "Model identifier to use when the task does not specify one (e.g. claude-sonnet-4).",
				Required: false,
			},
			{
				Key:       "api_key",
				Label:     "API Key",
				Type:      "secret",
				Help:      "Anthropic API key for the Claude Code CLI. Leave empty to use the CLI's own auth.",
				Required:  false,
				Sensitive: true,
			},
		},
	}
}

func (a *Adapter) ValidateConfig(blob json.RawMessage) error {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.ValidateConfig")
	if len(blob) == 0 || string(blob) == "null" || string(blob) == "{}" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(blob, &m); err != nil {
		return fmt.Errorf("claude-code config: invalid JSON: %w", err)
	}
	allowed := map[string]bool{"binary_path": true, "default_model": true, "api_key": true}
	for k := range m {
		if !allowed[k] {
			return fmt.Errorf("claude-code config: unknown key %q", k)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Prober
// ---------------------------------------------------------------------------

func (a *Adapter) Probe(_ context.Context, binaryPath string, _ time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.Probe", "binary", binaryPath)
	return "scaffold-0.0.0", binaryPath, nil
}

// ---------------------------------------------------------------------------
// ModelLister
// ---------------------------------------------------------------------------

func (a *Adapter) ListModels(_ context.Context, binaryPath string, _ time.Duration) ([]runner.ModelInfo, string, error) {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.ListModels", "binary", binaryPath)
	return []runner.ModelInfo{
		{ID: "claude-sonnet-4", Label: "Claude Sonnet 4"},
		{ID: "claude-opus-4", Label: "Claude Opus 4"},
	}, binaryPath, nil
}

// ---------------------------------------------------------------------------
// FailureClassifier
// ---------------------------------------------------------------------------

func (a *Adapter) ClassifyFailure(_ string) (kind, standardizedMsg string) {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.ClassifyFailure")
	return "", ""
}

// ---------------------------------------------------------------------------
// MetricsLabeler
// ---------------------------------------------------------------------------

func (a *Adapter) MetricsLabels(req runner.Request) map[string]string {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.MetricsLabels")
	return map[string]string{"model": a.EffectiveModel(req)}
}

// ---------------------------------------------------------------------------
// CycleMetaProvider
// ---------------------------------------------------------------------------

func (a *Adapter) CycleMeta(req runner.Request) map[string]any {
	slog.Debug("trace", "cmd", adapterLogCmd, "operation", "claudecode.Adapter.CycleMeta")
	return map[string]any{
		"claude_model_intent":    strings.TrimSpace(req.CursorModel),
		"claude_model_effective": a.EffectiveModel(req),
	}
}

// ---------------------------------------------------------------------------
// Compile-time assertions
// ---------------------------------------------------------------------------

var (
	_ runner.Runner               = (*Adapter)(nil)
	_ runner.ConfigSchemaProvider = (*Adapter)(nil)
	_ runner.ConfigValidator      = (*Adapter)(nil)
	_ runner.Prober               = (*Adapter)(nil)
	_ runner.ModelLister          = (*Adapter)(nil)
	_ runner.FailureClassifier    = (*Adapter)(nil)
	_ runner.MetricsLabeler       = (*Adapter)(nil)
	_ runner.CycleMetaProvider    = (*Adapter)(nil)
)
