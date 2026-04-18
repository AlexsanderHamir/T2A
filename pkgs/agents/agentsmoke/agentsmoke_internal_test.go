package agentsmoke

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifySucceeded_passesWhenTargetWrittenExactly(t *testing.T) {
	f := NewFixture(t)
	mustWrite(t, f.TargetPath(), f.ExpectedContents())

	if err := f.verifySucceeded(); err != nil {
		t.Fatalf("verifySucceeded: %v", err)
	}
}

func TestVerifySucceeded_failsWhenTargetMissing(t *testing.T) {
	f := NewFixture(t)

	err := f.verifySucceeded()
	if err == nil {
		t.Fatalf("verifySucceeded: expected error for missing target, got nil")
	}
	if !strings.Contains(err.Error(), "read target") {
		t.Fatalf("error = %v, want it to mention read target", err)
	}
}

func TestVerifySucceeded_failsWhenContentsMismatch(t *testing.T) {
	f := NewFixture(t)
	mustWrite(t, f.TargetPath(), "nope")

	err := f.verifySucceeded()
	if err == nil {
		t.Fatalf("verifySucceeded: expected error for wrong contents, got nil")
	}
	if !strings.Contains(err.Error(), "contents mismatch") {
		t.Fatalf("error = %v, want it to mention contents mismatch", err)
	}
}

func TestVerifySucceeded_failsWhenContentsHaveTrailingJunk(t *testing.T) {
	f := NewFixture(t)
	// Trailing extra newline is the most plausible "near miss" Cursor
	// might produce; assert we reject it so the smoke is not lulled
	// into accepting "almost right" output.
	mustWrite(t, f.TargetPath(), f.ExpectedContents()+"\n")

	if err := f.verifySucceeded(); err == nil {
		t.Fatalf("verifySucceeded: trailing junk must be rejected")
	}
}

func TestVerifySucceeded_failsWhenSiblingFilePresent(t *testing.T) {
	f := NewFixture(t)
	mustWrite(t, f.TargetPath(), f.ExpectedContents())
	mustWrite(t, filepath.Join(f.WorkingDir(), "leak.txt"), "x")

	err := f.verifySucceeded()
	if err == nil {
		t.Fatalf("verifySucceeded: extra sibling file must be rejected")
	}
	if !strings.Contains(err.Error(), "leak.txt") {
		t.Fatalf("error = %v, want it to name the extra file", err)
	}
}

func TestVerifySucceeded_failsWhenExtraFileInSubdir(t *testing.T) {
	f := NewFixture(t)
	mustWrite(t, f.TargetPath(), f.ExpectedContents())

	sub := filepath.Join(f.WorkingDir(), "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	mustWrite(t, filepath.Join(sub, "nested.txt"), "x")

	err := f.verifySucceeded()
	if err == nil {
		t.Fatalf("verifySucceeded: nested extra file must be rejected")
	}
	if !strings.Contains(err.Error(), "nested.txt") {
		t.Fatalf("error = %v, want it to name the nested extra file", err)
	}
}

func TestVerifyNotMutated_passesOnPristineDir(t *testing.T) {
	f := NewFixture(t)

	if err := f.verifyNotMutated(); err != nil {
		t.Fatalf("verifyNotMutated: %v", err)
	}
}

func TestVerifyNotMutated_failsWhenAnyFileExists(t *testing.T) {
	f := NewFixture(t)
	mustWrite(t, f.TargetPath(), "x")

	err := f.verifyNotMutated()
	if err == nil {
		t.Fatalf("verifyNotMutated: expected error for non-empty workdir, got nil")
	}
	if !strings.Contains(err.Error(), targetFilename) {
		t.Fatalf("error = %v, want it to name the offending file", err)
	}
}

func TestNewFixture_workingDirIsEmptyOnConstruction(t *testing.T) {
	f := NewFixture(t)

	entries, err := os.ReadDir(f.WorkingDir())
	if err != nil {
		t.Fatalf("read workdir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("fresh fixture workdir non-empty: %v", entries)
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
