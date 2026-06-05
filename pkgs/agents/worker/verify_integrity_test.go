package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// verify_integrity_test.go is a white-box unit test for the integrity
// snapshot helper. It pins the safety properties the verify pipeline
// relies on: HEAD-ref tampering, working-tree mutations outside the
// allowed verify-report path, post-snapshot errors, and the non-git
// bypass. Each case is a single assertion against the pure functions
// in verify_integrity.go — no worker/queue/store dependencies.

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed; skipping integrity test")
	}
}

func TestIntegritySnapshot_NonGitDir_Bypass(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	snap, err := captureIntegritySnapshot(context.Background(), dir)
	if err != nil {
		t.Fatalf("non-git snapshot err = %v, want nil", err)
	}
	if !snap.notGitRepo {
		t.Fatalf("expected notGitRepo=true on a plain dir, got snap=%+v", snap)
	}
}

func TestIntegritySnapshot_CleanRepo_NoChanges(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)

	pre, err := captureIntegritySnapshot(context.Background(), dir)
	if err != nil {
		t.Fatalf("pre snapshot: %v", err)
	}
	post, err := captureIntegritySnapshot(context.Background(), dir)
	if err != nil {
		t.Fatalf("post snapshot: %v", err)
	}
	diff := diffIntegritySnapshots(pre, post)
	if diff.headChanged {
		t.Errorf("head changed unexpectedly: pre=%s post=%s", pre.head, post.head)
	}
	if len(diff.addedPaths) != 0 {
		t.Errorf("addedPaths = %v, want empty", diff.addedPaths)
	}
}

// TestIntegrityDiff_AnyRepoRootChange_Tampered pins PR1's tightened
// contract: the worker now writes report files outside RepoRoot, so
// the integrity-check whitelist is empty by design. The previous
// .t2a/<cycleID>/verify-report.json allowance no longer exists — any
// new path inside the porcelain diff is tampering, even if it would
// once have been a legitimate report write.
func TestIntegrityDiff_AnyRepoRootChange_Tampered(t *testing.T) {
	cycleID := "abc123"
	pre := integritySnapshot{head: "deadbeef", changed: map[string]struct{}{}}
	post := integritySnapshot{
		head: "deadbeef",
		changed: map[string]struct{}{
			".t2a/" + cycleID + "/verify-report.json": {},
		},
	}
	tampered, summary := classifyIntegrityDiff(diffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Errorf("any RepoRoot change must be tampered after PR1; summary=%q", summary)
	}
}

func TestIntegrityDiff_OtherPathChanged_Tampered(t *testing.T) {
	cycleID := "abc123"
	pre := integritySnapshot{head: "deadbeef", changed: map[string]struct{}{}}
	post := integritySnapshot{
		head: "deadbeef",
		changed: map[string]struct{}{
			"src/main.go": {},
		},
	}
	tampered, summary := classifyIntegrityDiff(diffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Fatalf("expected tampered=true on src/main.go change, got false")
	}
	if !strings.Contains(summary, "src/main.go") {
		t.Errorf("summary missing path; got %q", summary)
	}
}

func TestIntegrityDiff_HeadChanged_Tampered(t *testing.T) {
	cycleID := "abc123"
	pre := integritySnapshot{head: "deadbeef", changed: map[string]struct{}{}}
	post := integritySnapshot{head: "cafe1234", changed: map[string]struct{}{}}
	tampered, summary := classifyIntegrityDiff(diffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Fatalf("HEAD ref change must trip integrity")
	}
	if !strings.Contains(summary, "HEAD") {
		t.Errorf("summary should mention HEAD; got %q", summary)
	}
}

func TestIntegrityDiff_ManyPaths_TruncatesSummary(t *testing.T) {
	cycleID := "abc"
	post := integritySnapshot{head: "x", changed: map[string]struct{}{}}
	for i := 0; i < 10; i++ {
		post.changed["path"+itoa(i)] = struct{}{}
	}
	pre := integritySnapshot{head: "x", changed: map[string]struct{}{}}
	tampered, summary := classifyIntegrityDiff(diffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Fatal("expected tampered")
	}
	if !strings.Contains(summary, "+5 more") {
		t.Errorf("summary should truncate with +5 more, got %q", summary)
	}
}

// TestIntegrityCheck_PreErrorIsTampered pins the fail-safe stance:
// even when the post-snapshot would have been clean, a failed
// pre-snapshot must mark the cycle tampered. A safety property cannot
// be defeated by "the check threw an exception".
func TestIntegrityCheck_PreErrorIsTampered(t *testing.T) {
	w := &Worker{options: Options{WorkingDir: t.TempDir()}}
	gitInit(t, w.options.WorkingDir)
	pre, _ := captureIntegritySnapshot(context.Background(), w.options.WorkingDir)
	tampered, reason := w.checkVerifyIntegrity(context.Background(), "c", pre, os.ErrPermission)
	if !tampered {
		t.Fatalf("pre-snapshot error must be treated as tampered; got reason=%q", reason)
	}
}

// TestIntegrityCheck_PostGitGoneIsTampered pins the .git-deletion
// case: pre saw a git repo, post sees none → tampered.
func TestIntegrityCheck_PostGitGoneIsTampered(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	w := &Worker{options: Options{WorkingDir: dir}}
	pre, err := captureIntegritySnapshot(context.Background(), dir)
	if err != nil || pre.notGitRepo {
		t.Fatalf("pre setup: err=%v notGitRepo=%v", err, pre.notGitRepo)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("rm .git: %v", err)
	}
	tampered, reason := w.checkVerifyIntegrity(context.Background(), "c", pre, nil)
	if !tampered {
		t.Fatalf(".git removal during verify must be tampered; got reason=%q", reason)
	}
}
