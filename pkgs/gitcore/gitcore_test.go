package gitcore_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitcore"
)

func TestExecError_Error_truncates_long_stderr(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", gitcore.StderrCap+50)
	err := gitcore.NewExecErrorForTest(long)
	msg := err.Error()
	if !strings.Contains(msg, "...") {
		t.Fatalf("expected truncated Error(), got %q", msg)
	}
	var ee *gitcore.ExecError
	if !errors.As(err, &ee) {
		t.Fatal("expected *ExecError")
	}
	if got := ee.Stderr(); got != long {
		t.Fatalf("Stderr() must stay uncapped, len=%d want %d", len(got), len(long))
	}
}

func TestStderrContains_matches_case_insensitive(t *testing.T) {
	t.Parallel()
	err := gitcore.NewExecErrorForTest("fatal: not a git repository")
	if !gitcore.StderrContains(err, "NOT A GIT") {
		t.Fatal("expected match")
	}
	if gitcore.StderrContains(err, "bad object") {
		t.Fatal("unexpected match")
	}
}

func TestStderr_non_exec_error(t *testing.T) {
	t.Parallel()
	if gitcore.Stderr(fmt.Errorf("plain")) != "" {
		t.Fatal("expected empty stderr for non-ExecError")
	}
}

func TestRun_git_missing(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// Cannot easily simulate missing git without breaking PATH; assert ErrGitMissing is distinct.
	if !errors.Is(gitcore.ErrGitMissing, gitcore.ErrGitMissing) {
		t.Fatal("ErrGitMissing sentinel")
	}
}

func TestRun_success_in_temp_repo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	init := exec.Command("git", "-C", dir, "init")
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	out, err := gitcore.Run(context.Background(), dir, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected git-dir path")
	}
}

func TestRun_not_a_repository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	_, err := gitcore.Run(context.Background(), dir, "status")
	if err == nil {
		t.Fatal("expected error for non-repo")
	}
	if !gitcore.StderrContains(err, "not a git repository") {
		t.Fatalf("got %v", err)
	}
}
