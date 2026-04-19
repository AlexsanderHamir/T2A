package registry_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/registry"
)

// TestList_pinsRegisteredRunners pins the catalogue exposed to the SPA.
// Adding a runner is intentionally a contract-changing event: this test
// must be updated alongside the new descriptor so reviewers see the
// surface change.
func TestList_pinsRegisteredRunners(t *testing.T) {
	got := registry.List()
	if len(got) != 1 {
		t.Fatalf("registered runners = %d, want 1 (only cursor today): %+v", len(got), got)
	}
	if got[0].ID != registry.CursorRunnerID {
		t.Errorf("[0].ID = %q, want %q", got[0].ID, registry.CursorRunnerID)
	}
	if got[0].Label != registry.CursorRunnerLabel {
		t.Errorf("[0].Label = %q, want %q", got[0].Label, registry.CursorRunnerLabel)
	}
	if got[0].DefaultBinaryHint != registry.CursorDefaultBinaryHint {
		t.Errorf("[0].DefaultBinaryHint = %q, want %q", got[0].DefaultBinaryHint, registry.CursorDefaultBinaryHint)
	}
}

// TestList_returnsFreshSlice covers the documented contract that each
// caller gets an isolated copy — defensive so an SPA-payload mutation
// doesn't leak into the next request.
func TestList_returnsFreshSlice(t *testing.T) {
	a := registry.List()
	a[0].Label = "mutated"
	b := registry.List()
	if b[0].Label == "mutated" {
		t.Error("List() returned aliased slice; mutation leaked across calls")
	}
}

func TestLookup_unknownReturnsErrUnknownRunner(t *testing.T) {
	_, err := registry.Lookup("not-a-runner")
	if !errors.Is(err, registry.ErrUnknownRunner) {
		t.Fatalf("err = %v, want ErrUnknownRunner", err)
	}
}

func TestBuild_cursorReturnsAdapter(t *testing.T) {
	r, err := registry.Build(registry.CursorRunnerID, registry.BuildOptions{
		BinaryPath: "cursor-agent",
		Version:    "1.2.3-test",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if r == nil {
		t.Fatal("build returned nil runner")
	}
	if r.Name() == "" {
		t.Error("Name() empty")
	}
	if r.Version() != "1.2.3-test" {
		t.Errorf("Version() = %q, want 1.2.3-test", r.Version())
	}
}

// TestBuild_emptyBinaryFallsBackToDefault pins the documented "empty
// path means default hint" behavior. Without it the supervisor would
// have to special-case the empty-binary path, duplicating logic.
func TestBuild_emptyBinaryFallsBackToDefault(t *testing.T) {
	_, err := registry.Build(registry.CursorRunnerID, registry.BuildOptions{})
	if err != nil {
		t.Fatalf("build with empty binary: %v", err)
	}
}

func TestBuild_unknownReturnsErrUnknownRunner(t *testing.T) {
	_, err := registry.Build("zzz", registry.BuildOptions{})
	if !errors.Is(err, registry.ErrUnknownRunner) {
		t.Fatalf("err = %v, want ErrUnknownRunner", err)
	}
}

// TestProbe_unknownRunnerSurfacesErrUnknownRunner exercises the error
// path without spawning a real binary. The happy path requires a real
// cursor-agent and is covered by the gated real-cursor smoke tests.
func TestProbe_unknownRunnerSurfacesErrUnknownRunner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := registry.Probe(ctx, "zzz", "", 50*time.Millisecond)
	if !errors.Is(err, registry.ErrUnknownRunner) {
		t.Fatalf("err = %v, want ErrUnknownRunner", err)
	}
}

// TestProbe_missingBinarySurfacesError confirms a probe against a path
// that cannot be exec'd returns a wrapped error (not a panic). We use
// a path that definitely doesn't exist.
func TestProbe_missingBinarySurfacesError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, _, err := registry.Probe(ctx, registry.CursorRunnerID, "/nonexistent/path/cursor-agent-xyz-9999", 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected error probing nonexistent binary")
	}
	if !strings.Contains(err.Error(), "cursor probe") {
		t.Logf("error: %v", err)
	}
}
