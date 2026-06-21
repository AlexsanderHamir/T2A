package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegritySnapshot_NonGitDir_Bypass(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	snap, err := CaptureIntegritySnapshot(context.Background(), NewExecRepo(), dir)
	if err != nil {
		t.Fatalf("non-git snapshot err = %v, want nil", err)
	}
	if !snap.NotGitRepo {
		t.Fatalf("expected NotGitRepo=true on a plain dir, got snap=%+v", snap)
	}
}

func TestIntegritySnapshot_CleanRepo_NoChanges(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	repo := NewExecRepo()

	pre, err := CaptureIntegritySnapshot(context.Background(), repo, dir)
	if err != nil {
		t.Fatalf("pre snapshot: %v", err)
	}
	post, err := CaptureIntegritySnapshot(context.Background(), repo, dir)
	if err != nil {
		t.Fatalf("post snapshot: %v", err)
	}
	diff := DiffIntegritySnapshots(pre, post)
	if diff.HeadChanged {
		t.Errorf("head changed unexpectedly: pre=%s post=%s", pre.Head, post.Head)
	}
	if len(diff.AddedPaths) != 0 {
		t.Errorf("addedPaths = %v, want empty", diff.AddedPaths)
	}
}

func TestIntegrityDiff_AnyRepoRootChange_Tampered(t *testing.T) {
	cycleID := "abc123"
	pre := IntegritySnapshot{Head: "deadbeef", Changed: map[string]struct{}{}}
	post := IntegritySnapshot{
		Head: "deadbeef",
		Changed: map[string]struct{}{
			".legacy-scratch/" + cycleID + "/verify-report.json": {},
		},
	}
	tampered, summary := ClassifyIntegrityDiff(DiffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Errorf("any RepoRoot change must be tampered after PR1; summary=%q", summary)
	}
}

func TestIntegrityDiff_OtherPathChanged_Tampered(t *testing.T) {
	cycleID := "abc123"
	pre := IntegritySnapshot{Head: "deadbeef", Changed: map[string]struct{}{}}
	post := IntegritySnapshot{
		Head: "deadbeef",
		Changed: map[string]struct{}{
			"src/main.go": {},
		},
	}
	tampered, summary := ClassifyIntegrityDiff(DiffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Fatalf("expected tampered=true on src/main.go change, got false")
	}
	if !strings.Contains(summary, "src/main.go") {
		t.Errorf("summary missing path; got %q", summary)
	}
}

func TestIntegrityDiff_HeadChanged_Tampered(t *testing.T) {
	cycleID := "abc123"
	pre := IntegritySnapshot{Head: "deadbeef", Changed: map[string]struct{}{}}
	post := IntegritySnapshot{Head: "cafe1234", Changed: map[string]struct{}{}}
	tampered, summary := ClassifyIntegrityDiff(DiffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Fatalf("HEAD ref change must trip integrity")
	}
	if !strings.Contains(summary, "HEAD") {
		t.Errorf("summary should mention HEAD; got %q", summary)
	}
}

func TestIntegrityDiff_ManyPaths_TruncatesSummary(t *testing.T) {
	cycleID := "abc"
	post := IntegritySnapshot{Head: "x", Changed: map[string]struct{}{}}
	for i := 0; i < 10; i++ {
		post.Changed["path"+Itoa(i)] = struct{}{}
	}
	pre := IntegritySnapshot{Head: "x", Changed: map[string]struct{}{}}
	tampered, summary := ClassifyIntegrityDiff(DiffIntegritySnapshots(pre, post), cycleID)
	if !tampered {
		t.Fatal("expected tampered")
	}
	if !strings.Contains(summary, "+5 more") {
		t.Errorf("summary should truncate with +5 more, got %q", summary)
	}
}

func TestIntegrityCheck_PreErrorIsTampered(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	repo := NewExecRepo()
	pre, _ := CaptureIntegritySnapshot(context.Background(), repo, dir)
	tampered, reason := CheckVerifyIntegrity(context.Background(), repo, dir, "c", pre, os.ErrPermission)
	if !tampered {
		t.Fatalf("pre-snapshot error must be treated as tampered; got reason=%q", reason)
	}
}

func TestIntegrityCheck_PostGitGoneIsTampered(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	repo := NewExecRepo()
	pre, err := CaptureIntegritySnapshot(context.Background(), repo, dir)
	if err != nil || pre.NotGitRepo {
		t.Fatalf("pre setup: err=%v NotGitRepo=%v", err, pre.NotGitRepo)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("rm .git: %v", err)
	}
	tampered, reason := CheckVerifyIntegrity(context.Background(), repo, dir, "c", pre, nil)
	if !tampered {
		t.Fatalf(".git removal during verify must be tampered; got reason=%q", reason)
	}
}
