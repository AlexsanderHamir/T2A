package agentsmoke

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const agentsmokeLogCmd = "taskapi"

// targetFilename is the canonical filename Cursor must create inside
// Fixture.WorkingDir. Pinned so the prompt and the assertion logic
// stay in lockstep across every stage of
// docs/AGENT-WORKER-SMOKE-PLAN.md.
const targetFilename = "agent-smoke-output.txt"

// expectedContents is the exact byte sequence Cursor must write into
// TargetPath: the literal "OK" followed by a single newline. Three
// bytes total. Chosen so the prompt can speak in plain text and the
// assertion can reduce to an exact bytes comparison.
const expectedContents = "OK\n"

// Fixture owns the tempdir, the canonical prompt, and the
// post-condition assertions for the real-cursor smoke. Construct via
// NewFixture; the tempdir is torn down automatically at the end of
// the test.
//
// The Fixture is intentionally inert: it does not start a runner, a
// worker, or an HTTP server. Stages 2 and 3 wire it into the real
// cursor.Adapter and the full taskapi stack respectively. Stage 1
// only exercises the assertion logic via a fake runner.
type Fixture struct {
	workingDir       string
	targetPath       string
	expectedContents string
	prompt           string
}

// NewFixture returns a Fixture rooted at t.TempDir(). The returned
// Fixture is safe to use for the rest of the test; teardown of the
// underlying directory is owned by the testing package.
func NewFixture(t *testing.T) *Fixture {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.NewFixture")
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, targetFilename)
	return &Fixture{
		workingDir:       dir,
		targetPath:       target,
		expectedContents: expectedContents,
		prompt:           buildPrompt(target),
	}
}

// buildPrompt returns the canonical prompt body. It is deliberately
// explicit about the absolute path and forbids tangential work so the
// post-condition assertion can be tight. See
// docs/AGENT-WORKER-SMOKE-PLAN.md "Why the prompt must be
// deterministic".
func buildPrompt(target string) string {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.buildPrompt", "target", target)
	return fmt.Sprintf(`Create a single file at exactly the absolute path:

  %s

The file must contain exactly three bytes and nothing else: the
ASCII letters 'O' and 'K' followed by a single newline character
('\n'). The file must not contain any other byte, no leading or
trailing whitespace, no BOM, no commentary.

Do not create any other file. Do not write any explanation file,
notes file, README, or scratch file anywhere on disk. Do not modify
any pre-existing file. The only side effect this task is allowed to
have is the creation of that one file with exactly those three
bytes.

When you are done, you may print a brief confirmation to stdout, but
the only state that matters is the file on disk.`, target)
}

// WorkingDir is the per-test tempdir. Pass it as
// runner.Request.WorkingDir (Stage 2) or via
// T2A_AGENT_WORKER_WORKING_DIR on the worker process (Stage 3).
func (f *Fixture) WorkingDir() string {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.WorkingDir")
	return f.workingDir
}

// Prompt is the canonical prompt body the runner receives on stdin.
// Pass it as runner.Request.Prompt (Stage 2) or as the task's
// initial_prompt on POST /tasks (Stage 3).
func (f *Fixture) Prompt() string {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.Prompt")
	return f.prompt
}

// TargetPath is the absolute path the prompt asks Cursor to create.
// Tests assert against this path; nothing else inside WorkingDir is
// permitted to exist after a successful run.
func (f *Fixture) TargetPath() string {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.TargetPath")
	return f.targetPath
}

// ExpectedContents is the exact byte sequence TargetPath must hold
// after a successful run. Three bytes: "OK\n".
func (f *Fixture) ExpectedContents() string {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.ExpectedContents")
	return f.expectedContents
}

// AssertSucceeded asserts the smoke post-condition: the target file
// exists with exactly ExpectedContents, and no other file was
// created anywhere inside WorkingDir. On failure it calls t.Fatalf
// with a diff so callers do not have to thread the error.
func (f *Fixture) AssertSucceeded(t *testing.T) {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.AssertSucceeded")
	t.Helper()
	if err := f.verifySucceeded(); err != nil {
		t.Fatalf("agentsmoke: %v", err)
	}
}

// AssertNotMutated asserts that WorkingDir is still completely empty.
// Used by negative tests to prove a failed runner did not touch disk
// at all.
func (f *Fixture) AssertNotMutated(t *testing.T) {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.AssertNotMutated")
	t.Helper()
	if err := f.verifyNotMutated(); err != nil {
		t.Fatalf("agentsmoke: %v", err)
	}
}

// verifySucceeded returns nil iff TargetPath holds exactly
// ExpectedContents and no other regular file exists inside
// WorkingDir. Exposed unexported so internal tests can exercise the
// assertion logic without going through testing.T.Fatalf.
func (f *Fixture) verifySucceeded() error {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.verifySucceeded")
	got, err := os.ReadFile(f.targetPath)
	if err != nil {
		return fmt.Errorf("read target %s: %w", f.targetPath, err)
	}
	if string(got) != f.expectedContents {
		return fmt.Errorf("target %s contents mismatch:\n  want (%d bytes): %q\n  got  (%d bytes): %q",
			f.targetPath, len(f.expectedContents), f.expectedContents, len(got), string(got))
	}
	extras, err := f.extraFiles()
	if err != nil {
		return err
	}
	if len(extras) > 0 {
		return fmt.Errorf("unexpected extra files in %s: %s",
			f.workingDir, strings.Join(extras, ", "))
	}
	return nil
}

// verifyNotMutated returns nil iff WorkingDir contains zero entries.
func (f *Fixture) verifyNotMutated() error {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.verifyNotMutated")
	entries, err := os.ReadDir(f.workingDir)
	if err != nil {
		return fmt.Errorf("read workdir %s: %w", f.workingDir, err)
	}
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return fmt.Errorf("expected pristine workdir %s, found: %s",
		f.workingDir, strings.Join(names, ", "))
}

// extraFiles walks WorkingDir and returns every regular file path
// (relative to WorkingDir, slash-separated) other than TargetPath.
// Detection of "extra" content is rooted at WorkingDir so a sibling
// file or a nested escape both fail the assertion.
func (f *Fixture) extraFiles() ([]string, error) {
	slog.Debug("trace", "cmd", agentsmokeLogCmd, "operation", "agentsmoke.Fixture.extraFiles")
	var extras []string
	walkErr := filepath.WalkDir(f.workingDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if p == f.targetPath {
			return nil
		}
		rel, relErr := filepath.Rel(f.workingDir, p)
		if relErr != nil {
			return relErr
		}
		extras = append(extras, filepath.ToSlash(rel))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk workdir %s: %w", f.workingDir, walkErr)
	}
	sort.Strings(extras)
	return extras, nil
}
