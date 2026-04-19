package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestOpenRoot_and_Resolve(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg", "x.go")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Abs() != dir {
		t.Fatalf("Abs = %q want %q", r.Abs(), dir)
	}
	abs, err := r.Resolve("pkg/x.go")
	if err != nil {
		t.Fatal(err)
	}
	if abs != sub {
		t.Fatalf("Resolve = %q want %q", abs, sub)
	}
	_, err = r.Resolve("../outside")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// TestRoot_Resolve_acceptsFilenamesContainingDoubleDot pins the contract
// that filenames with a ".." substring (e.g. "foo..bar.go", "..hidden",
// "eslint..config.js") are valid and must resolve, as long as no path
// component is literally "..". The previous early-reject
// `strings.Contains(rel, "..")` was overly broad and rejected such files
// despite the downstream `pathEscapesRoot(relOut)` check being the
// authoritative traversal guard. The traversal cases ("../outside",
// "a/../../b", "..") continue to be rejected — see the partner tests
// TestOpenRoot_and_Resolve and TestRoot_Resolve_rejectsTraversal.
func TestRoot_Resolve_acceptsFilenamesContainingDoubleDot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []string{
		"foo..bar.go",
		"eslint..config.js",
		"..gitkeep",
		"deep/dir/something..min.js",
	}
	for _, rel := range cases {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %q: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write for %q: %v", rel, err)
		}
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range cases {
		got, err := r.Resolve(rel)
		if err != nil {
			t.Errorf("Resolve(%q) unexpected error: %v", rel, err)
			continue
		}
		want := filepath.Join(dir, filepath.FromSlash(rel))
		if got != want {
			t.Errorf("Resolve(%q) = %q want %q", rel, got, want)
		}
	}
}

// TestRoot_Resolve_rejectsTraversal pins the symmetric contract: any
// path that resolves outside the root after Clean — whether via a
// leading "..", an embedded "a/../../b", or a bare ".." — must be
// rejected with ErrInvalidInput. This guards against the fix to
// TestRoot_Resolve_acceptsFilenamesContainingDoubleDot accidentally
// loosening the traversal guard.
func TestRoot_Resolve_rejectsTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"..",
		"../outside",
		"../../way/outside",
		"a/../../b",
		"a/b/../../../c",
	}
	for _, rel := range cases {
		_, err := r.Resolve(rel)
		if err == nil {
			t.Errorf("Resolve(%q) expected error, got nil", rel)
			continue
		}
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("Resolve(%q) expected ErrInvalidInput, got %v", rel, err)
		}
	}
}

func TestRoot_Ready_ok(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Ready(); err != nil {
		t.Fatal(err)
	}
}

func TestRoot_Ready_fails_when_directory_removed(t *testing.T) {
	dir := t.TempDir()
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := r.Ready(); err == nil {
		t.Fatal("expected error when root path is gone")
	}
}

func TestRoot_Search(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustMk := func(rel string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustMk("src/app.go")
	mustMk("lib/extra_foo.txt")
	mustMk(".git/HEAD")
	mustMk("node_modules/pkg/index.js")
	mustMk("vendor/v/mod.go")

	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty query lists tracked files skips special dirs", func(t *testing.T) {
		paths, err := r.Search("")
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			for _, prefix := range []string{".git/", "node_modules/", "vendor/"} {
				if strings.HasPrefix(p, prefix) || strings.Contains(p, "/"+prefix) {
					t.Fatalf("unexpected path under skipped dir: %q in %v", p, paths)
				}
			}
		}
		got := make(map[string]bool)
		for _, p := range paths {
			got[p] = true
		}
		for _, want := range []string{"src/app.go", "lib/extra_foo.txt"} {
			if !got[want] {
				t.Fatalf("missing %q in %v", want, paths)
			}
		}
	})

	t.Run("substring case insensitive", func(t *testing.T) {
		paths, err := r.Search("FOO")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 1 || paths[0] != "lib/extra_foo.txt" {
			t.Fatalf("paths = %v want [lib/extra_foo.txt]", paths)
		}
	})

	t.Run("no match", func(t *testing.T) {
		paths, err := r.Search("zzznonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Fatalf("paths = %v want []", paths)
		}
	})
}

func TestLineCount_and_ValidateRange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := LineCount(p)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("LineCount = %d", n)
	}
	if err := ValidateRange(p, 1, 3); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRange(p, 2, 4); err == nil {
		t.Fatal("expected error for past EOF")
	}
}

func TestValidateRange_invalidBounds_checked_before_file_size(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "huge.txt")
	huge := strings.Repeat("a", maxFileReadBytes+1)
	if err := os.WriteFile(p, []byte(huge), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateRange(p, 0, 1)
	if err == nil {
		t.Fatal("expected invalid input error")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if !strings.Contains(err.Error(), "line numbers must be >= 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLineCount_rejects_files_larger_than_limit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	big := strings.Repeat("a", maxFileReadBytes+1)
	if err := os.WriteFile(p, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LineCount(p)
	if err == nil {
		t.Fatal("expected file too large error")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidatePromptMentions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("l1\nl2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ValidatePromptMentions(`ref @ok.go(1-2)`); err != nil {
		t.Fatal(err)
	}
	err = r.ValidatePromptMentions(`ref @ok.go(1-9)`)
	if err == nil {
		t.Fatal("expected error for bad range")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// TestValidatePromptMentions_singlePrefix pins the "single tasks: invalid input
// prefix on the wire" invariant for every error path inside
// repo.Root.ValidatePromptMentions. Before this fix every mention failure
// whose underlying cause itself wrapped domain.ErrInvalidInput (i.e. r.Resolve
// rejections, validateRangeBounds rejections, LineCount rejections,
// validateRangeWithLineCount rejections) produced
// "tasks: invalid input: mention @<path>: tasks: invalid input: <reason>"
// — a doubled prefix the docs explicitly called out as an "implementation
// detail" caveat (docs/API-HTTP.md POST /tasks repo-mention validation
// section). The wrapMention helper strips the inner prefix so the wire phrase
// always carries it exactly once. errors.Is(err, domain.ErrInvalidInput)
// must still hold for the 400 mapping in
// pkgs/tasks/handler/handler_http_json.go::storeErrorClientMessage.
//
// Cases covered:
//   - resolveEscape: r.Resolve fails with "invalid path" / "path escapes
//     repo root" — every escape path inside Resolve re-wraps ErrInvalidInput,
//     historically the worst offender for double-wrapping.
//   - rangeOutOfBounds: validateRangeBounds rejects (1,0) with "end line must
//     be >= start line" — wraps ErrInvalidInput inside repo/range_validation.go.
//   - rangeStartZero: validateRangeBounds rejects (0,1) with "line numbers
//     must be >= 1" — same wrap.
//   - rangeBeyondFileLength: validateRangeWithLineCount rejects (1,99) on a
//     2-line file — wraps ErrInvalidInput at the post-LineCount boundary.
//
// Skip cases (not double-wrapped before the fix):
//   - file-does-not-exist: synthesized literal string in this package, never
//     had a doubled prefix.
//   - directory-not-file: synthesized literal string in this package, never
//     had a doubled prefix.
func TestValidatePromptMentions_singlePrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "two.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		prompt string
	}{
		{"resolveEscape", "see @../escape.txt"},
		{"rangeOutOfBounds", "see @two.txt(1-0)"},
		{"rangeStartZero", "see @two.txt(0-1)"},
		{"rangeBeyondFileLength", "see @two.txt(1-99)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.ValidatePromptMentions(tc.prompt)
			if err == nil {
				t.Fatalf("expected error for %q", tc.prompt)
			}
			msg := err.Error()
			n := strings.Count(msg, domain.ErrInvalidInput.Error())
			if n != 1 {
				t.Fatalf("error=%q has %d occurrences of %q want exactly 1 (wire prefix must not double-wrap)", msg, n, domain.ErrInvalidInput.Error())
			}
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("error=%q must still satisfy errors.Is(domain.ErrInvalidInput) so handler 400 mapping fires", msg)
			}
			if !strings.Contains(msg, "@") {
				t.Fatalf("error=%q must include the offending @<path> mention substring (docs/API-HTTP.md contract)", msg)
			}
		})
	}
}

// TestValidatePromptMentions_singlePrefix_synthesized covers the two cases
// whose reasons are literal strings synthesized inside ValidatePromptMentions
// (rather than borrowed from Resolve/LineCount/range validators). These never
// double-wrapped before the fix but they share the wrapMentionMsg path with
// the wrapped cases, so a future copy-edit that switched back to direct
// fmt.Errorf would silently drift only the wrapped cases. Pinning both
// halves of the format keeps wrapMentionMsg honest as the single source of
// truth for the wire phrase.
func TestValidatePromptMentions_singlePrefix_synthesized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		prompt string
		want   string
	}{
		{"missingFile", "see @nope.txt", "file does not exist"},
		{"directoryNotFile", "see @subdir", "path is a directory, not a file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.ValidatePromptMentions(tc.prompt)
			if err == nil {
				t.Fatalf("expected error for %q", tc.prompt)
			}
			msg := err.Error()
			if n := strings.Count(msg, domain.ErrInvalidInput.Error()); n != 1 {
				t.Fatalf("error=%q has %d prefix occurrences want 1", msg, n)
			}
			if !strings.Contains(msg, tc.want) {
				t.Fatalf("error=%q missing reason %q", msg, tc.want)
			}
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("error=%q must satisfy errors.Is(domain.ErrInvalidInput)", msg)
			}
		})
	}
}

func TestRootResolve_rejects_symlink_escape(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "escape.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve("escape.txt")
	if err == nil {
		t.Fatal("expected symlink escape error")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRootResolve_allows_symlink_inside_root(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "pkg", "x.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "x.go")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	r, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := r.Resolve("x.go")
	if err != nil {
		t.Fatal(err)
	}
	if abs != linkPath {
		t.Fatalf("Resolve = %q want %q", abs, linkPath)
	}
}
