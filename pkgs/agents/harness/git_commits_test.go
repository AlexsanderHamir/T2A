package harness

import (
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestFormatGitContextForPrompt_omitsWorktreeWhenSameAsRepo(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	got := formatGitContextForPrompt([]domain.TaskCycleCommit{
		{
			Seq: 1, Repo: "/repo", Worktree: "/repo", Branch: "main",
			SHA: "abc1234567890abcdef1234567890abcdef1234", CommittedAt: when, Message: "feat",
		},
	})
	if !strings.Contains(got, "Repo:") || strings.Contains(got, "Worktree:") {
		t.Fatalf("expected repo-only context, got %q", got)
	}
	if !strings.Contains(got, "abc1234") {
		t.Fatalf("expected short sha in prompt, got %q", got)
	}
}

func TestFormatGitContextForPrompt_emptyReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := formatGitContextForPrompt(nil); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
