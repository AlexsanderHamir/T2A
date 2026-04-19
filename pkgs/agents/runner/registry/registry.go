package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
)

const registryLogCmd = "taskapi"

// Descriptor describes one runner choice exposed to the SPA settings
// page. ID is the persisted enum value (matches AppSettings.Runner);
// Label is the human-readable display name; DefaultBinaryHint is the
// suggested binary name when the operator hasn't picked one (empty
// means PATH lookup of ID itself).
type Descriptor struct {
	ID                string
	Label             string
	DefaultBinaryHint string
}

// BuildOptions configures the runner adapter at construction time.
// BinaryPath is the operator-chosen path (empty means use the
// descriptor's DefaultBinaryHint); Version is the string returned by
// Probe (recorded in TaskCyclePhase MetaJSON for the audit trail).
type BuildOptions struct {
	BinaryPath string
	Version    string
}

// ErrUnknownRunner is returned when the supervisor or handler asks
// for a runner id the registry does not recognise. Callers map this
// to a 400 in HTTP handlers.
var ErrUnknownRunner = errors.New("registry: unknown runner")

// CursorRunnerID is the only runner id registered today. Exported so
// the store layer's default and the SPA settings serializer can
// reference the same string constant.
const CursorRunnerID = "cursor"

// CursorRunnerLabel is the human-readable name shown in the SPA
// settings <select>. Pinned here so the label stays consistent with
// the descriptor and any future renames go through one site.
const CursorRunnerLabel = "Cursor CLI"

// CursorDefaultBinaryHint is the placeholder shown in the SPA when
// the operator has not chosen a custom cursor binary path. The empty
// path resolves against PATH at supervisor probe time.
const CursorDefaultBinaryHint = "cursor-agent"

// List returns every registered runner descriptor, sorted by ID for
// stable rendering in the SPA. The returned slice is a fresh copy so
// callers can mutate it without affecting later calls.
func List() []Descriptor {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.List")
	out := []Descriptor{
		{ID: CursorRunnerID, Label: CursorRunnerLabel, DefaultBinaryHint: CursorDefaultBinaryHint},
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Lookup returns the descriptor for id, or ErrUnknownRunner.
func Lookup(id string) (Descriptor, error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Lookup", "id", id)
	id = strings.TrimSpace(id)
	for _, d := range List() {
		if d.ID == id {
			return d, nil
		}
	}
	return Descriptor{}, fmt.Errorf("%w: %q", ErrUnknownRunner, id)
}

// Build constructs a runner.Runner for id with the supplied options.
// Returns ErrUnknownRunner when id is not registered. Empty
// BinaryPath falls back to the descriptor's DefaultBinaryHint so the
// caller never has to special-case "use the default".
func Build(id string, opts BuildOptions) (runner.Runner, error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Build",
		"id", id, "binary", opts.BinaryPath, "version", opts.Version)
	desc, err := Lookup(id)
	if err != nil {
		return nil, err
	}
	bin := strings.TrimSpace(opts.BinaryPath)
	if bin == "" {
		bin = desc.DefaultBinaryHint
	}
	switch desc.ID {
	case CursorRunnerID:
		return cursor.New(cursor.Options{
			BinaryPath: bin,
			Version:    opts.Version,
		}), nil
	default:
		// Defensive: List + Lookup should make this unreachable, but
		// returning the typed sentinel keeps the handler error mapping
		// uniform if a future descriptor is added without a Build branch.
		return nil, fmt.Errorf("%w: %q (no Build branch)", ErrUnknownRunner, desc.ID)
	}
}

// Probe runs the runner's --version probe with a bounded deadline and
// returns the trimmed version string plus the absolute binary path that
// was actually executed (PATH-resolved when the operator left the field
// blank, so the SPA "Test cursor binary" success message can show
// "auto-detected on PATH at /usr/local/bin/cursor-agent" instead of
// just "OK"). Used by the supervisor at boot (to populate
// runner.Version()) and by POST /settings/probe-cursor (to validate a
// cursor binary path before saving).
//
// Empty binaryPath uses the descriptor's DefaultBinaryHint. timeout
// <= 0 falls back to the runner's documented default. The resolved
// path is best-effort: when LookPath fails the original input is
// returned so the caller still has something to display alongside the
// probe error.
func Probe(ctx context.Context, id, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Probe",
		"id", id, "binary", binaryPath, "timeout_ns", int64(timeout))
	desc, err := Lookup(id)
	if err != nil {
		return "", "", err
	}
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = desc.DefaultBinaryHint
	}
	switch desc.ID {
	case CursorRunnerID:
		resolved := cursor.ResolveBinaryPath(bin)
		v, probeErr := cursor.Probe(ctx, resolved, timeout, nil)
		return v, resolved, probeErr
	default:
		return "", "", fmt.Errorf("%w: %q (no Probe branch)", ErrUnknownRunner, desc.ID)
	}
}
