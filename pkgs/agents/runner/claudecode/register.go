package claudecode

import (
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry"
)

const (
	RunnerID          = "claude-code"
	RunnerLabel       = "Claude Code CLI"
	DefaultBinaryHint = "claude"
)

func init() {
	slog.Debug("trace", "cmd", "claudecode", "operation", "agents.runner.claudecode.register")
	registry.Register(
		registry.Descriptor{
			ID:                RunnerID,
			Label:             RunnerLabel,
			DefaultBinaryHint: DefaultBinaryHint,
		},
		func(opts registry.BuildOptions) (runner.Runner, error) {
			return New(Options{
				BinaryPath:   opts.BinaryPath,
				Version:      opts.Version,
				DefaultModel: opts.CursorModel,
			}), nil
		},
	)
}
