package harness

import (
	"os/exec"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/resume"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestComposeContinuationPrompt_scopeLockAndAntiDiscovery(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cycle := &domain.TaskCycle{ID: "child-1", AttemptSeq: 2, StartedAt: started}
	bundle := &ContinuationBundle{
		LineageAttempt: 1,
		FailureClass:   resume.FailureClassVerify,
		FailureReason:  verificationFailedReason,
		FailurePhase:   domain.PhaseVerify,
		ScopeFiles:     []string{"pkgs/foo/bar.go"},
		Commits: []domain.TaskCycleCommit{
			{SHA: "abc1234567890abcdef1234567890abcdef1234", Message: "prior work"},
		},
	}
	got := prompt.ComposeContinuation("base prompt", continuationInputFromBundle(cycle, bundle))
	for _, frag := range []string{
		"Continuation", "Scope lock", "pkgs/foo/bar.go", "abc123456789", "base prompt",
	} {
		if !containsSubstr(got, frag) {
			t.Fatalf("missing %q in prompt:\n%s", frag, got)
		}
	}
}

func TestAppendResumeNotice_andCommitPolicy(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cycle := &domain.TaskCycle{ID: "cycle-1", StartedAt: started}
	known := []domain.TaskCycleCommit{
		{Seq: 1, SHA: "abc123def456", Message: "feat: add health check"},
	}
	promptText := prompt.AppendResumeNotice("base", cycle, domain.PhaseExecute, known)
	for _, frag := range []string{"Worker resume notice", "cycle-1", "abc123def456", "base"} {
		if !containsSubstr(promptText, frag) {
			t.Fatalf("resume notice missing %q in %q", frag, promptText)
		}
	}
	withCommit := prompt.AppendGitCommitPolicy("", false)
	if !containsSubstr(withCommit, "Git commits (required)") || !containsSubstr(withCommit, "commits[]") {
		t.Fatalf("commit policy missing required block: %q", withCommit)
	}
	if containsSubstr(withCommit, "git rev-list") {
		t.Fatalf("commit policy must not mention rev-list discovery: %q", withCommit)
	}
	if containsSubstr(withCommit, "t2a:cycle") {
		t.Fatalf("commit policy must not mention t2a markers: %q", withCommit)
	}
	resumePolicy := prompt.AppendGitCommitPolicy("", true)
	for _, frag := range []string{"new commits only", "already indexed"} {
		if !containsSubstr(resumePolicy, frag) {
			t.Fatalf("resume commit policy missing %q in %q", frag, resumePolicy)
		}
	}
	opRetry := prompt.AppendOperatorRetryResumeNotice("base", cycle, known)
	for _, frag := range []string{"Operator retry", "cycle-1", "abc123def456", "commits[]", "base"} {
		if !containsSubstr(opRetry, frag) {
			t.Fatalf("operator retry notice missing %q in %q", frag, opRetry)
		}
	}
	dir := t.TempDir()
	initGitRepoForDiffTest(t, dir)
	diff := verifyDiffSection(dir)
	if containsSubstr(diff, "(diff unavailable") {
		t.Fatalf("verify diff unavailable for git repo: %q", diff)
	}
}

func initGitRepoForDiffTest(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init"},
	} {
		if err := exec.Command("git", append([]string{"-C", dir}, args...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
}

func containsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
