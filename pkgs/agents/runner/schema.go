package runner

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrCapabilityNotSupported is returned when a caller asks for a
// capability (probe, list-models, failure classification) that the
// adapter does not implement. Handlers map this to HTTP 501.
var ErrCapabilityNotSupported = errors.New("runner: capability not supported")

// ConfigSchema describes the configuration surface an adapter exposes
// to the SPA settings page and to PATCH /settings validation. The SPA
// renders a form from Fields; the handler calls ValidateConfig before
// persisting the blob.
type ConfigSchema struct {
	Version int           `json:"version"`
	Fields  []ConfigField `json:"fields"`
}

// ConfigField is one operator-visible knob in the adapter's config.
type ConfigField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // "string" | "secret" | "int" | "bool" | "enum"
	Default  any    `json:"default,omitempty"`
	Help     string `json:"help,omitempty"`
	Required bool   `json:"required,omitempty"`
	// Sensitive fields are masked in the SPA and redacted in logs.
	Sensitive  bool        `json:"sensitive,omitempty"`
	EnumValues []EnumValue `json:"enum_values,omitempty"`
}

// EnumValue is one option for a ConfigField with Type "enum".
type EnumValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ModelInfo describes one model returned by a ModelLister capability.
type ModelInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ---------------------------------------------------------------------------
// Optional capability interfaces. Adapters opt in by implementing them;
// the registry detects support via type assertion.
// ---------------------------------------------------------------------------

// Prober is implemented by adapters that can verify a CLI binary is
// usable (e.g. `cursor --version`). The supervisor calls this at boot
// and on POST /runners/{id}/probe.
type Prober interface {
	Probe(ctx context.Context, binaryPath string, timeout time.Duration) (version, resolvedBin string, err error)
}

// ModelLister is implemented by adapters whose CLI can enumerate
// available models (e.g. `cursor-agent --list-models`). Called by
// POST /runners/{id}/list-models.
type ModelLister interface {
	ListModels(ctx context.Context, binaryPath string, timeout time.Duration) ([]ModelInfo, string, error)
}

// FailureClassifier is implemented by adapters that can recognise
// known failure patterns in CLI output and return a stable
// (failure_kind, standardized_message) pair for the SPA and audit
// trail. The worker calls this after a non-zero-exit run.
type FailureClassifier interface {
	ClassifyFailure(combined string) (kind, standardizedMsg string)
}

// ---------------------------------------------------------------------------
// Extended runner methods. These are added to the Runner interface so
// every adapter surfaces its own config schema, metrics labels, and
// cycle metadata without the generic layer knowing adapter specifics.
// ---------------------------------------------------------------------------

// ConfigSchemaProvider returns the adapter's configuration schema.
// The SPA renders a form from this; the handler validates PATCH
// payloads against it.
type ConfigSchemaProvider interface {
	ConfigSchema() ConfigSchema
}

// ConfigValidator validates an opaque config blob against the
// adapter's schema. Returns nil when valid; a descriptive error
// otherwise. Called by the handler before persisting PATCH /settings.
type ConfigValidator interface {
	ValidateConfig(blob json.RawMessage) error
}

// MetricsLabeler returns adapter-owned extra Prometheus labels for a
// given request. The cursor adapter returns {"model": effective} so
// the by-model series stays compatible. Adapters without model
// selection return an empty map. Called from the worker on the hot
// path; implementations MUST be pure (no I/O).
type MetricsLabeler interface {
	MetricsLabels(req Request) map[string]string
}

// CycleMetaProvider returns adapter-owned key-value pairs to include
// in TaskCycle.MetaJSON. The cursor adapter returns
// {"cursor_model_intent": ..., "cursor_model_effective": ...} so the
// audit trail stays backward-compatible. Called once per cycle start;
// implementations MUST be pure (no I/O).
type CycleMetaProvider interface {
	CycleMeta(req Request) map[string]any
}
