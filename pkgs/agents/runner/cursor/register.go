package cursor

import (
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry"
)

func init() {
	slog.Debug("trace", "cmd", "cursor", "operation", "agents.runner.cursor.register")
	registry.Register(
		registry.Descriptor{
			ID:                registry.CursorRunnerID,
			Label:             registry.CursorRunnerLabel,
			DefaultBinaryHint: registry.CursorDefaultBinaryHint,
		},
		func(opts registry.BuildOptions) (runner.Runner, error) {
			return New(Options{
				BinaryPath:         opts.BinaryPath,
				Version:            opts.Version,
				DefaultCursorModel: strings.TrimSpace(opts.CursorModel),
			}), nil
		},
	)
}
