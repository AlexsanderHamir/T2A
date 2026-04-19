//go:build cursor_real

// This file is the operator-run smoke for the real cursor-agent
// binary. It is excluded from default builds by the cursor_real
// build tag and additionally gated by the T2A_TEST_REAL_CURSOR=1
// env var so even with the tag set the test no-ops unless the
// operator opted in. See docs/AGENT-WORKER.md "Smoke run" for the
// operator runbook and pkgs/agents/agentsmoke/doc.go for the
// prompt + assertion rationale.
//
// Run it locally as:
//
//	$env:T2A_TEST_REAL_CURSOR='1'
//	$env:T2A_TEST_CURSOR_BIN='C:\path\to\cursor-agent.cmd' # optional override
//	go test -tags=cursor_real -run TestCursorAdapter_RealBinary -race ./pkgs/agents/runner/cursor/... -count=1
//
// Prerequisites: cursor-agent on PATH (or the env override) and
// Cursor logged in. The test owns a fresh tempdir; teardown is
// automatic.

package cursor_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/agentsmoke"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/cursor"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// realCursorRunGateEnv is the on/off switch for the real-binary
// smoke. Even with the cursor_real build tag set, the test skips
// unless this env var is exactly "1" so a stray go test invocation
// against the package never triggers a paid Cursor run.
const realCursorRunGateEnv = "T2A_TEST_REAL_CURSOR"

// realCursorBinaryEnv lets operators point the smoke at a specific
// cursor-agent binary (for example the .cmd shim on Windows). When
// unset the adapter's default ("cursor-agent" resolved against PATH)
// is used.
const realCursorBinaryEnv = "T2A_TEST_CURSOR_BIN"

// realCursorRunBudget is the per-call wall-clock cap. Generous
// because cold caches + first-tool-call latency can take well over a
// minute in practice; still well under the worker's
// DefaultRunTimeout so this test never masks a genuine worker hang.
const realCursorRunBudget = 90 * time.Second

func TestCursorAdapter_RealBinary_writesExpectedFile(t *testing.T) {
	if os.Getenv(realCursorRunGateEnv) != "1" {
		t.Skipf("skipping: %s != 1; this test invokes a paid Cursor run", realCursorRunGateEnv)
	}

	binaryPath := os.Getenv(realCursorBinaryEnv)
	if binaryPath == "" {
		binaryPath = "cursor-agent"
	}

	probeCtx, probeCancel := context.WithTimeout(context.Background(), cursor.DefaultProbeTimeout)
	defer probeCancel()
	version, probeErr := cursor.Probe(probeCtx, binaryPath, cursor.DefaultProbeTimeout, nil)
	if probeErr != nil {
		t.Fatalf("cursor probe %q failed: %v\nHint: install cursor-agent and set %s to its path",
			binaryPath, probeErr, realCursorBinaryEnv)
	}
	t.Logf("cursor-agent version: %s (binary=%s)", version, binaryPath)

	fixture := agentsmoke.NewFixture(t)
	adapter := cursor.New(cursor.Options{
		BinaryPath: binaryPath,
		Version:    version,
	})

	runCtx, runCancel := context.WithTimeout(context.Background(), realCursorRunBudget)
	defer runCancel()

	res, runErr := adapter.Run(runCtx, runner.Request{
		TaskID:     "task-real-cursor-smoke",
		AttemptSeq: 1,
		Phase:      domain.PhaseExecute,
		Prompt:     fixture.Prompt(),
		WorkingDir: fixture.WorkingDir(),
		Timeout:    realCursorRunBudget,
	})
	if runErr != nil {
		t.Fatalf("cursor.Adapter.Run failed: %v\nSummary: %s\nRawOutput tail:\n%s",
			runErr, res.Summary, tailLines(res.RawOutput, 40))
	}
	if res.Status != domain.PhaseStatusSucceeded {
		t.Fatalf("Status = %q, want succeeded\nSummary: %s\nRawOutput tail:\n%s",
			res.Status, res.Summary, tailLines(res.RawOutput, 40))
	}
	if !strings.Contains(strings.ToLower(res.Summary), "ok") &&
		!strings.Contains(strings.ToLower(res.Summary), "created") &&
		!strings.Contains(strings.ToLower(res.Summary), "wrote") {
		// Soft signal only; the file post-condition below is the
		// authoritative assertion. Log so a confused human gets a
		// hint when the model phrases its summary unexpectedly.
		t.Logf("note: cursor summary did not mention created/wrote/OK: %q", res.Summary)
	}

	fixture.AssertSucceeded(t)
}

// tailLines returns the last n lines of s, joined by newlines, so the
// failure message is operator-readable without dumping the entire
// (already-redacted) RawOutput payload.
func tailLines(s string, n int) string {
	if s == "" {
		return "<empty>"
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return fmt.Sprintf("... (%d earlier lines elided)\n%s",
		len(lines)-n, strings.Join(lines[len(lines)-n:], "\n"))
}
