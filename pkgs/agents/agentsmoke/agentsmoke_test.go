package agentsmoke_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/agentsmoke"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestFixture_Prompt_referencesAbsoluteTargetPath(t *testing.T) {
	f := agentsmoke.NewFixture(t)

	if !filepath.IsAbs(f.TargetPath()) {
		t.Fatalf("TargetPath must be absolute, got %q", f.TargetPath())
	}
	if !strings.Contains(f.Prompt(), f.TargetPath()) {
		t.Fatalf("Prompt does not mention TargetPath %q.\nPrompt:\n%s",
			f.TargetPath(), f.Prompt())
	}
}

func TestFixture_TargetPath_isInsideWorkingDir(t *testing.T) {
	f := agentsmoke.NewFixture(t)

	rel, err := filepath.Rel(f.WorkingDir(), f.TargetPath())
	if err != nil {
		t.Fatalf("rel(%q, %q): %v", f.WorkingDir(), f.TargetPath(), err)
	}
	if strings.HasPrefix(rel, "..") {
		t.Fatalf("TargetPath %q escapes WorkingDir %q (rel=%q)",
			f.TargetPath(), f.WorkingDir(), rel)
	}
}

func TestFixture_ExpectedContents_isThreeBytes(t *testing.T) {
	f := agentsmoke.NewFixture(t)

	if got, want := f.ExpectedContents(), "OK\n"; got != want {
		t.Fatalf("ExpectedContents = %q, want %q", got, want)
	}
}

// TestFixture_HappyPath_withFakeRunner exercises the integration
// shape Stages 2 and 3 will use: a runner is invoked with WorkingDir
// set to fixture.WorkingDir(), the prompt body comes from
// fixture.Prompt(), and once the workspace ends up in the expected
// state, AssertSucceeded recognises it as a green run.
//
// runnerfake does not actually mutate disk; this test stands in for
// that by writing the expected file inline before asserting. Stage 2
// swaps the inline write for a real cursor-agent invocation.
func TestFixture_HappyPath_withFakeRunner(t *testing.T) {
	f := agentsmoke.NewFixture(t)

	r := runnerfake.New()
	const taskID = "task-smoke-happy"
	r.Script(taskID, domain.PhaseExecute, runner.Result{
		Status:  domain.PhaseStatusSucceeded,
		Summary: "smoke ok",
	})

	res, err := r.Run(context.Background(), runner.Request{
		TaskID:     taskID,
		Phase:      domain.PhaseExecute,
		Prompt:     f.Prompt(),
		WorkingDir: f.WorkingDir(),
	})
	if err != nil {
		t.Fatalf("fake run: %v", err)
	}
	if res.Status != domain.PhaseStatusSucceeded {
		t.Fatalf("fake status = %v, want succeeded", res.Status)
	}

	calls := r.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].WorkingDir != f.WorkingDir() {
		t.Fatalf("WorkingDir = %q, want %q", calls[0].WorkingDir, f.WorkingDir())
	}
	if calls[0].Prompt != f.Prompt() {
		t.Fatalf("Prompt mismatch: runner did not see the fixture prompt")
	}

	if err := os.WriteFile(f.TargetPath(), []byte(f.ExpectedContents()), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	f.AssertSucceeded(t)
}
