package claudecode_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/claudecode"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/registry"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapter_registersInRegistry(t *testing.T) {
	t.Parallel()
	desc, err := registry.Lookup(claudecode.RunnerID)
	require.NoError(t, err)
	assert.Equal(t, "claude-code", desc.ID)
	assert.Equal(t, "Claude Code CLI", desc.Label)
	assert.Equal(t, "claude", desc.DefaultBinaryHint)
}

func TestAdapter_appearsInList(t *testing.T) {
	t.Parallel()
	all := registry.List()
	found := false
	for _, d := range all {
		if d.ID == claudecode.RunnerID {
			found = true
			break
		}
	}
	assert.True(t, found, "claude-code should appear in registry.List()")
}

func TestAdapter_Build(t *testing.T) {
	t.Parallel()
	r, err := registry.Build(claudecode.RunnerID, registry.BuildOptions{
		BinaryPath: "/usr/local/bin/claude",
		Version:    "1.0.0",
	})
	require.NoError(t, err)
	assert.Equal(t, "claude-code", r.Name())
	assert.Equal(t, "1.0.0", r.Version())
}

func TestAdapter_RunReturnsErrTimeout(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{})
	res, err := a.Run(context.Background(), runner.Request{TaskID: "t1"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, runner.ErrTimeout))
	assert.Equal(t, domain.PhaseStatusFailed, res.Status)
}

func TestAdapter_EffectiveModel(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{DefaultModel: "claude-sonnet-4"})
	assert.Equal(t, "claude-sonnet-4", a.EffectiveModel(runner.Request{}))
	assert.Equal(t, "override", a.EffectiveModel(runner.Request{CursorModel: "override"}))
}

func TestAdapter_ConfigSchema(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{})
	schema := a.ConfigSchema()
	assert.Equal(t, 1, schema.Version)
	assert.Len(t, schema.Fields, 3)
	keys := make([]string, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		keys = append(keys, f.Key)
	}
	assert.ElementsMatch(t, []string{"binary_path", "default_model", "api_key"}, keys)
}

func TestAdapter_ValidateConfig(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{})

	assert.NoError(t, a.ValidateConfig(nil))
	assert.NoError(t, a.ValidateConfig(json.RawMessage(`{}`)))
	assert.NoError(t, a.ValidateConfig(json.RawMessage(`{"binary_path":"/usr/bin/claude"}`)))
	assert.Error(t, a.ValidateConfig(json.RawMessage(`{"unknown_key":"x"}`)))
	assert.Error(t, a.ValidateConfig(json.RawMessage(`not json`)))
}

func TestAdapter_Probe(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{})
	ver, bin, err := a.Probe(context.Background(), "/usr/bin/claude", 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "scaffold-0.0.0", ver)
	assert.Equal(t, "/usr/bin/claude", bin)
}

func TestAdapter_ListModels(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{})
	models, bin, err := a.ListModels(context.Background(), "claude", 5*time.Second)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "claude", bin)
	assert.Equal(t, "claude-sonnet-4", models[0].ID)
}

func TestAdapter_MetricsLabels(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{DefaultModel: "claude-opus-4"})
	labels := a.MetricsLabels(runner.Request{})
	assert.Equal(t, "claude-opus-4", labels["model"])
}

func TestAdapter_CycleMeta(t *testing.T) {
	t.Parallel()
	a := claudecode.New(claudecode.Options{DefaultModel: "claude-sonnet-4"})
	meta := a.CycleMeta(runner.Request{CursorModel: "claude-opus-4"})
	assert.Equal(t, "claude-opus-4", meta["claude_model_intent"])
	assert.Equal(t, "claude-opus-4", meta["claude_model_effective"])
}

func TestAdapter_RegistryProbe(t *testing.T) {
	t.Parallel()
	ver, bin, err := registry.Probe(context.Background(), claudecode.RunnerID, "claude", 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "scaffold-0.0.0", ver)
	assert.Equal(t, "claude", bin)
}

func TestAdapter_RegistryListModels(t *testing.T) {
	t.Parallel()
	models, bin, err := registry.ListModelsForRunner(context.Background(), claudecode.RunnerID, "claude", 5*time.Second)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "claude", bin)
}
