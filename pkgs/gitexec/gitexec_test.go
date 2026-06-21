package gitexec_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitexec"
)

func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "note.txt")
	runGit(t, dir, "commit", "-m", "initial")
	out := runGit(t, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(out)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestShowCommitPatch_returns_diff_for_known_sha(t *testing.T) {
	dir := t.TempDir()
	sha := initGitRepo(t, dir)

	patch, truncated, err := gitexec.ShowCommitPatch(context.Background(), dir, sha, gitexec.DefaultMaxPatchBytes)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("expected full patch")
	}
	if !strings.Contains(patch, "diff --git") {
		t.Fatalf("patch %#v", patch)
	}
}

func TestShowCommitPatch_not_found(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	_, _, err := gitexec.ShowCommitPatch(context.Background(), dir, "deadbeef", gitexec.DefaultMaxPatchBytes)
	if err != gitexec.ErrNotFound {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestShowCommitPatch_truncates(t *testing.T) {
	dir := t.TempDir()
	sha := initGitRepo(t, dir)

	_, truncated, err := gitexec.ShowCommitPatch(context.Background(), dir, sha, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("expected truncated")
	}
}

func TestLoadCommitMeta_returns_author_and_shortstat(t *testing.T) {
	dir := t.TempDir()
	sha := initGitRepo(t, dir)

	meta, err := gitexec.LoadCommitMeta(context.Background(), dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Author != "Test" || meta.AuthorEmail != "t@example.com" {
		t.Fatalf("author %#v", meta)
	}
	if meta.FilesChanged < 1 || meta.Insertions < 1 {
		t.Fatalf("shortstat %#v", meta)
	}
}
