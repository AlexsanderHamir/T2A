package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func TestMatchReportedSHAInAncestry_exactAndAbbreviated(t *testing.T) {
	t.Parallel()
	ancestry := []string{
		"aa2e05b31bc583c6f54fd0673bb12879ed2edc45",
		"26ff8c16c0e5de8e5ba64c1ce2ce6ecbfdeef4be",
	}
	got, err := matchReportedSHAInAncestry("aa2e05b", ancestry)
	if err != nil {
		t.Fatalf("abbreviated: %v", err)
	}
	if got != ancestry[0] {
		t.Fatalf("got %q want %q", got, ancestry[0])
	}
	got, err = matchReportedSHAInAncestry(strings.ToUpper(ancestry[1]), ancestry)
	if err != nil {
		t.Fatalf("exact case-insensitive: %v", err)
	}
	if got != ancestry[1] {
		t.Fatalf("got %q want %q", got, ancestry[1])
	}
}

func TestMatchReportedSHAInAncestry_rejectsOutsideAncestry(t *testing.T) {
	t.Parallel()
	ancestry := []string{"aa2e05b31bc583c6f54fd0673bb12879ed2edc45"}
	_, err := matchReportedSHAInAncestry("deadbeef", ancestry)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestMatchReportedSHAInAncestry_rejectsAmbiguousPrefix(t *testing.T) {
	t.Parallel()
	ancestry := []string{
		"aaaa111111111111111111111111111111111111",
		"aaaa222222222222222222222222222222222222",
	}
	_, err := matchReportedSHAInAncestry("aaaa", ancestry)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("want ambiguous detail, got %v", err)
	}
}

// Regression (2026-06-19): execute_invalid_commit when criteria-report listed
// 7-char SHAs that matched real cycle commits but failed exact-string cross-check.
func TestResolvePhaseCommits_acceptsAbbreviatedReportedSHA(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)

	ctx := context.Background()
	base, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse base: %v", err)
	}

	name := fmt.Sprintf("work-%d.txt", time.Now().UnixNano())
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("work"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"add", name},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "-m", "cycle work"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	head, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse head: %v", err)
	}
	short := head
	if len(short) > 7 {
		short = short[:7]
	}

	entries, err := resolvePhaseCommits(ctx, gitPhaseContext{
		Repo:         dir,
		Worktree:     dir,
		CycleBaseSHA: base,
		BaseBranch:   "main",
	}, []commitReport{{SHA: short, Branch: "main"}}, "cycle-test")
	if err != nil {
		t.Fatalf("resolvePhaseCommits: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: got %d want 1", len(entries))
	}
	if entries[0].SHA != head {
		t.Fatalf("SHA: got %q want %q", entries[0].SHA, head)
	}
	if entries[0].Branch != "main" {
		t.Fatalf("branch: got %q want main", entries[0].Branch)
	}
}

// Regression (2026-06-18): resume attempt listed parent-cycle SHAs in criteria-report;
// empty rev-list + stale reported SHA must not fail execute_invalid_commit.
func TestResolvePhaseCommits_ignoresReportedOutsideAncestry(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)

	ctx := context.Background()
	base, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse base: %v", err)
	}

	entries, err := resolvePhaseCommits(ctx, gitPhaseContext{
		Repo: dir, Worktree: dir, CycleBaseSHA: base, BaseBranch: "main",
	}, []commitReport{{SHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Branch: "main"}}, "cycle-stale")
	if err != nil {
		t.Fatalf("resolvePhaseCommits: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries: got %d want 0 (no new commits in range)", len(entries))
	}
}

func TestBuildInheritedCommitEntries_copiesParentIndexedCommits(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)

	ctx := context.Background()

	name := fmt.Sprintf("inherit-%d.txt", time.Now().UnixNano())
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"add", name},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "-m", "parent work"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse head: %v", err)
	}

	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	inherited := []domain.TaskCycleCommit{
		{Seq: 1, Repo: dir, Worktree: dir, Branch: "main", SHA: head, CommittedAt: when, Message: "parent work"},
	}
	entries, err := buildInheritedCommitEntries(ctx, gitPhaseContext{
		Repo: dir, Worktree: dir, BaseBranch: "main", CycleBaseSHA: head,
	}, inherited, 2)
	if err != nil {
		t.Fatalf("buildInheritedCommitEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: got %d want 1", len(entries))
	}
	if entries[0].SHA != head || entries[0].PhaseSeq != 2 {
		t.Fatalf("entry: %+v", entries[0])
	}
}
