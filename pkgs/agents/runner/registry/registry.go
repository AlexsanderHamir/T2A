package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
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

// Factory creates a runner.Runner from build options. Each adapter
// registers one Factory alongside its Descriptor; the registry calls
// it from Build.
type Factory func(opts BuildOptions) (runner.Runner, error)

// BuildOptions configures the runner adapter at construction time.
// BinaryPath is the operator-chosen path (empty means use the
// descriptor's DefaultBinaryHint); Version is the string returned by
// Probe (recorded in TaskCyclePhase MetaJSON for the audit trail).
type BuildOptions struct {
	BinaryPath string
	Version    string
	// CursorModel is forwarded only for the cursor runner: non-empty values
	// become `cursor-agent --model <value>`; empty means omit the flag.
	CursorModel string
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

// ---------------------------------------------------------------------------
// Self-registration infrastructure
// ---------------------------------------------------------------------------

type registration struct {
	desc    Descriptor
	factory Factory
}

var (
	mu       sync.RWMutex
	adapters = map[string]registration{}
)

// Register adds a runner adapter to the global registry. Adapters
// call this from an init() function in a dedicated register.go file;
// the cmd/taskapi binary imports registry/all to trigger all inits.
// Last-write-wins for a given ID so tests can override registrations.
func Register(desc Descriptor, factory Factory) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Register",
		"id", desc.ID)
	mu.Lock()
	defer mu.Unlock()
	adapters[desc.ID] = registration{desc: desc, factory: factory}
}

// List returns every registered runner descriptor, sorted by ID for
// stable rendering in the SPA. The returned slice is a fresh copy so
// callers can mutate it without affecting later calls.
func List() []Descriptor {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.List")
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Descriptor, 0, len(adapters))
	for _, reg := range adapters {
		out = append(out, reg.desc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Lookup returns the descriptor for id, or ErrUnknownRunner.
func Lookup(id string) (Descriptor, error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Lookup", "id", id)
	id = strings.TrimSpace(id)
	mu.RLock()
	reg, ok := adapters[id]
	mu.RUnlock()
	if !ok {
		return Descriptor{}, fmt.Errorf("%w: %q", ErrUnknownRunner, id)
	}
	return reg.desc, nil
}

// Build constructs a runner.Runner for id with the supplied options.
// Returns ErrUnknownRunner when id is not registered. Empty
// BinaryPath falls back to the descriptor's DefaultBinaryHint so the
// caller never has to special-case "use the default".
func Build(id string, opts BuildOptions) (runner.Runner, error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Build",
		"id", id, "binary", opts.BinaryPath, "version", opts.Version)
	id = strings.TrimSpace(id)
	mu.RLock()
	reg, ok := adapters[id]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRunner, id)
	}
	bin := strings.TrimSpace(opts.BinaryPath)
	if bin == "" {
		bin = reg.desc.DefaultBinaryHint
	}
	opts.BinaryPath = bin
	return reg.factory(opts)
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
// Probe builds a temporary runner via the registered factory, then
// type-asserts to runner.Prober. Adapters that do not implement Prober
// return ErrCapabilityNotSupported.
func Probe(ctx context.Context, id, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.Probe",
		"id", id, "binary", binaryPath, "timeout_ns", int64(timeout))
	id = strings.TrimSpace(id)
	mu.RLock()
	reg, ok := adapters[id]
	mu.RUnlock()
	if !ok {
		return "", "", fmt.Errorf("%w: %q", ErrUnknownRunner, id)
	}
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = reg.desc.DefaultBinaryHint
	}

	r, buildErr := reg.factory(BuildOptions{BinaryPath: bin})
	if buildErr != nil {
		return "", "", fmt.Errorf("probe build %q: %w", id, buildErr)
	}
	prober, ok := r.(runner.Prober)
	if !ok {
		return "", "", fmt.Errorf("probe %q: %w", id, runner.ErrCapabilityNotSupported)
	}
	return prober.Probe(ctx, bin, timeout)
}

// ListModelsForRunner builds a temporary runner and delegates to its
// ModelLister capability. Returns ErrCapabilityNotSupported when the
// adapter does not implement runner.ModelLister.
func ListModelsForRunner(ctx context.Context, id, binaryPath string, timeout time.Duration) ([]runner.ModelInfo, string, error) {
	slog.Debug("trace", "cmd", registryLogCmd, "operation", "agents.runner.registry.ListModelsForRunner",
		"id", id, "binary", binaryPath, "timeout_ns", int64(timeout))
	id = strings.TrimSpace(id)
	mu.RLock()
	reg, ok := adapters[id]
	mu.RUnlock()
	if !ok {
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownRunner, id)
	}
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = reg.desc.DefaultBinaryHint
	}

	r, buildErr := reg.factory(BuildOptions{BinaryPath: bin})
	if buildErr != nil {
		return nil, "", fmt.Errorf("list-models build %q: %w", id, buildErr)
	}
	lister, ok := r.(runner.ModelLister)
	if !ok {
		return nil, "", fmt.Errorf("list-models %q: %w", id, runner.ErrCapabilityNotSupported)
	}
	return lister.ListModels(ctx, bin, timeout)
}
