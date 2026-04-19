package cursor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ListModelsTimeout is the wall-clock cap for cursor-agent --list-models.
// Listing can be slower than --version because it may query Cursor services.
const ListModelsTimeout = 30 * time.Second

// DefaultListModelsBinary is used when the operator leaves the binary path
// empty, matching registry.CursorDefaultBinaryHint.
const DefaultListModelsBinary = "cursor-agent"

// ModelInfo is one entry from `cursor-agent --list-models` stdout.
type ModelInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ListModels runs `<binary> --list-models` with a bounded deadline and
// parses the CLI's human-readable table (lines like "id - Label").
func ListModels(ctx context.Context, binaryPath string, timeout time.Duration, run ProbeFn) ([]ModelInfo, string, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.ListModels",
		"binary", binaryPath, "timeout_ns", int64(timeout))
	p := strings.TrimSpace(binaryPath)
	if p == "" {
		p = DefaultListModelsBinary
	}
	resolved := ResolveBinaryPath(p)
	if resolved == "" {
		return nil, "", errors.New("cursor list-models: could not resolve binary path")
	}
	if timeout <= 0 {
		timeout = ListModelsTimeout
	}
	if run == nil {
		run = DefaultProbeFn
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout, stderr, exitCode, err := run(runCtx, resolved, "--list-models")
	if err != nil {
		if runCtx.Err() != nil {
			return nil, resolved, fmt.Errorf("cursor list-models %q: timed out after %s: %w", resolved, timeout, err)
		}
		return nil, resolved, fmt.Errorf("cursor list-models %q: exec failed: %w", resolved, err)
	}
	if exitCode != 0 {
		return nil, resolved, fmt.Errorf("cursor list-models %q: exit %d (stderr=%q)", resolved, exitCode, trimForLog(stderr))
	}
	out := parseListModelsOutput(stdout)
	if len(out) == 0 {
		return nil, resolved, fmt.Errorf("cursor list-models %q: no models parsed from output", resolved)
	}
	return out, resolved, nil
}

func parseListModelsOutput(stdout []byte) []ModelInfo {
	var out []ModelInfo
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "available models" || strings.HasPrefix(lower, "available models") {
			continue
		}
		idx := strings.Index(line, " - ")
		if idx <= 0 {
			continue
		}
		id := strings.TrimSpace(line[:idx])
		label := strings.TrimSpace(line[idx+len(" - "):])
		if id == "" {
			continue
		}
		out = append(out, ModelInfo{ID: id, Label: label})
	}
	return out
}
